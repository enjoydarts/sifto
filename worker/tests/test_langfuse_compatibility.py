"""Exercise the installed SDK and the common, strictly defined v3/v4 API."""

import asyncio
import json
from contextlib import contextmanager, nullcontext
from types import SimpleNamespace
from unittest.mock import patch
from uuid import uuid4

import pytest
from langfuse import Langfuse
from opentelemetry import baggage, context
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from starlette.requests import Request
from starlette.responses import Response

from app import main
from app.services import langfuse_client


class CommonClient:
    """No dynamic Mock attributes or removed SDK methods can hide API mistakes."""

    def __init__(self, observation_context=None):
        self.observation = SimpleNamespace(update=lambda **kwargs: None, score=lambda **kwargs: None)
        self.observation_context = observation_context or nullcontext(self.observation)
        self.starts = []

    def start_as_current_observation(self, *, name, as_type, input=None, metadata=None):
        self.starts.append({"name": name, "as_type": as_type, "input": input, "metadata": metadata})
        return self.observation_context


@pytest.fixture
def real_sdk():
    exporter = InMemorySpanExporter()
    provider = TracerProvider()
    # Keep the SDK's real propagation processor, replacing only its network exporter.
    with patch("langfuse._client.span_processor.OTLPSpanExporter", return_value=exporter):
        client = Langfuse(
            public_key=f"pk-offline-{uuid4()}",
            secret_key="sk-offline",
            base_url="http://offline.invalid",
            tracer_provider=provider,
        )
        with patch.object(langfuse_client, "_client", return_value=client):
            yield client, exporter
        client.shutdown()
        provider.shutdown()


def exported(client, exporter):
    client.flush()
    return {span.name: span for span in exporter.get_finished_spans()}


@pytest.mark.parametrize("as_type", ["span", "generation"])
def test_common_observation_api_creates_both_types(as_type):
    client = CommonClient()
    with patch.object(langfuse_client, "_client", return_value=client):
        with langfuse_client.span("request", as_type=as_type) as observation:
            assert observation is client.observation
    assert client.starts[0]["as_type"] == as_type


def test_real_sdk_records_root_child_and_generation_details(real_sdk):
    client, exporter = real_sdk
    with langfuse_client.span(
        "root", user_id="user-a", session_id="item:a", tags=["worker", "purpose:test"],
        metadata={"path": "/extract-body"},
    ) as root:
        assert root is not None
        with langfuse_client.span("child", as_type="generation") as generation:
            assert generation is not None
            langfuse_client.update_current(
                model="test-model", usage_details={"input": 12, "output": 7},
                cost_details={"input": 0.001, "output": 0.002},
            )
    spans = exported(client, exporter)
    for name in ("root", "child"):
        assert spans[name].attributes["user.id"] == "user-a"
        assert spans[name].attributes["session.id"] == "item:a"
        assert spans[name].attributes["langfuse.trace.tags"] == ("worker", "purpose:test")
    assert spans["child"].parent.span_id == spans["root"].context.span_id
    assert spans["child"].attributes["langfuse.observation.model.name"] == "test-model"
    assert json.loads(spans["child"].attributes["langfuse.observation.usage_details"]) == {"input": 12, "output": 7}
    assert json.loads(spans["child"].attributes["langfuse.observation.cost_details"]) == {"input": 0.001, "output": 0.002}
    assert not baggage.get_all()


def test_real_sdk_scores_use_bound_observation_and_current_span_fallback(real_sdk):
    client, _exporter = real_sdk
    # Exercise real SDK score methods without enqueuing a network-bound score.
    with patch.object(client, "create_score", autospec=True) as create_score:
        with langfuse_client.span("scored", as_type="generation") as generation:
            langfuse_client.score_current("bound-quality", 1, data_type="NUMERIC")
            with langfuse_client.bind_span(None):
                langfuse_client.score_current("fallback-quality", 0.5, data_type="NUMERIC")
    assert create_score.call_count == 2
    for call, name in zip(create_score.call_args_list, ("bound-quality", "fallback-quality")):
        assert call.kwargs["name"] == name
        assert call.kwargs["trace_id"] == generation.trace_id
        assert call.kwargs["observation_id"] == generation.id


@pytest.mark.parametrize("failure", [None, ValueError("application failed"), asyncio.CancelledError()])
def test_real_sdk_nested_scope_restores_context_on_every_exit(real_sdk, failure):
    client, exporter = real_sdk
    original_context = context.get_current()
    with langfuse_client.span("outer", user_id="outer-user", session_id="source:outer", tags=["outer"]):
        outer_context = context.get_current()
        try:
            with langfuse_client.span("inner", user_id="inner-user", session_id="item:inner", tags=["inner"]):
                if failure is not None:
                    raise failure
        except BaseException as caught:
            assert caught is failure
        else:
            assert failure is None
        assert context.get_current() == outer_context
        with langfuse_client.span("after-inner"):
            pass
    assert context.get_current() == original_context
    assert langfuse_client._current_span_var.get() is None
    with langfuse_client.span("after-request"):
        pass
    spans = exported(client, exporter)
    for name in ("outer", "after-inner"):
        assert spans[name].attributes["user.id"] == "outer-user"
        assert spans[name].attributes["session.id"] == "source:outer"
        assert spans[name].attributes["langfuse.trace.tags"] == ("outer",)
    assert spans["inner"].attributes["user.id"] == "inner-user"
    assert "user.id" not in spans["after-request"].attributes
    assert "session.id" not in spans["after-request"].attributes
    assert "langfuse.trace.tags" not in spans["after-request"].attributes


def request(headers, path="/extract-body"):
    return Request({
        "type": "http", "method": "POST", "path": path,
        "headers": [(key.lower().encode(), value.encode()) for key, value in headers.items()],
    })


@pytest.mark.parametrize(
    "identifiers,session_id",
    [({"x-sifto-item-id": "i", "x-sifto-digest-id": "d", "x-sifto-source-id": "s"}, "item:i"),
     ({"x-sifto-digest-id": "d", "x-sifto-source-id": "s"}, "digest:d"),
     ({"x-sifto-source-id": "s"}, "source:s"), ({}, None)],
)
def test_middleware_scopes_normalized_request_attributes(identifiers, session_id):
    scopes = []

    @contextmanager
    def capture_span(name, **kwargs):
        scopes.append(kwargs)
        yield None

    async def respond(_request):
        return Response(status_code=204)

    headers = {"x-internal-worker-secret": "test-secret", "x-sifto-user-id": " user-a ",
               "x-sifto-purpose": "test", **identifiers}
    with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"), patch.object(main, "langfuse_span", capture_span):
        response = asyncio.run(main.langfuse_request_tracing(request(headers), respond))
    assert response.status_code == 204
    assert scopes[0]["user_id"] == "user-a"
    assert scopes[0]["session_id"] == session_id
    assert scopes[0]["tags"] == ["worker", "path:/extract-body", "purpose:test"]


def test_real_sdk_concurrent_requests_and_thread_children_are_isolated(real_sdk):
    client, exporter = real_sdk

    async def run_requests():
        arrived = 0
        both_arrived = asyncio.Event()

        async def run_request(user):
            async def respond(_request):
                nonlocal arrived
                arrived += 1
                if arrived == 2:
                    both_arrived.set()
                await both_arrived.wait()
                with langfuse_client.span(f"child-{user}", as_type="generation"):
                    await asyncio.sleep(0)
                def thread_child():
                    with langfuse_client.span(f"thread-{user}"):
                        pass
                await asyncio.to_thread(thread_child)
                return Response(status_code=200)
            await main.langfuse_request_tracing(request({
                "x-internal-worker-secret": "test-secret", "x-sifto-user-id": user,
                "x-sifto-item-id": user, "x-sifto-purpose": user,
            }), respond)

        await asyncio.gather(run_request("a"), run_request("b"))
        with langfuse_client.span("after-concurrent"):
            pass

    with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"):
        asyncio.run(run_requests())
    spans = exported(client, exporter)
    roots = [span for span in exporter.get_finished_spans() if span.name == "worker:extract-body"]
    assert len(roots) == 2
    for root in roots:
        user = root.attributes["user.id"]
        for span in (root, spans[f"child-{user}"], spans[f"thread-{user}"]):
            assert span.attributes["user.id"] == user
            assert span.attributes["session.id"] == f"item:{user}"
            assert span.attributes["langfuse.trace.tags"] == ("worker", "path:/extract-body", f"purpose:{user}")
            assert "test-secret" not in str(span.attributes)
        assert spans[f"child-{user}"].parent.span_id == root.context.span_id
    assert "user.id" not in spans["after-concurrent"].attributes


def test_sdk3_legacy_trace_fields_remain_updated():
    class LegacyClient(CommonClient):
        def update_current_trace(self, **kwargs):
            self.trace_attributes = kwargs

    client = LegacyClient()
    with patch.object(langfuse_client, "_client", return_value=client):
        with langfuse_client.span("request", user_id="user", session_id="item:i", tags=["worker"]):
            pass
    assert client.trace_attributes == {"user_id": "user", "session_id": "item:i", "tags": ["worker"]}


@pytest.mark.parametrize("attribute", [
    None,
    "not callable",
    property(lambda self: (_ for _ in ()).throw(RuntimeError("SDK property failed"))),
    lambda self, **kwargs: (_ for _ in ()).throw(RuntimeError("SDK trace update failed")),
])
def test_missing_noncallable_or_broken_legacy_method_is_safe(attribute):
    client_type = type("CurrentClient", (CommonClient,), {"update_current_trace": attribute})
    with patch.object(langfuse_client, "_client", return_value=client_type()):
        with langfuse_client.span("request", user_id="user") as observation:
            assert observation is not None


@pytest.mark.parametrize("stage", ["create", "enter", "exit"])
@pytest.mark.parametrize("failure", [None, ValueError("application failed"), asyncio.CancelledError()])
def test_sdk_context_failures_never_replace_application_control_flow(stage, failure):
    class FailingContext:
        def __enter__(self):
            if stage == "enter":
                raise RuntimeError("SDK enter failed")
            return SimpleNamespace()

        def __exit__(self, *_args):
            if stage == "exit":
                raise RuntimeError("SDK exit failed")
            return True  # The SDK must not suppress application errors either.

    class FailingClient(CommonClient):
        def start_as_current_observation(self, **kwargs):
            if stage == "create":
                raise RuntimeError("SDK creation failed")
            return FailingContext()

    with patch.object(langfuse_client, "_client", return_value=FailingClient()):
        try:
            with langfuse_client.span("request") as observation:
                assert (observation is None) == (stage != "exit")
                if failure is not None:
                    raise failure
        except BaseException as caught:
            assert caught is failure
        else:
            assert failure is None
    assert langfuse_client._current_span_var.get() is None


@pytest.mark.parametrize("stage", ["create", "enter", "exit"])
def test_attribute_context_failure_keeps_observation_and_request_alive(stage):
    class FailingAttributes:
        def __enter__(self):
            if stage == "enter":
                raise RuntimeError("SDK attribute entry failed")

        def __exit__(self, *_args):
            if stage == "exit":
                raise RuntimeError("SDK attribute exit failed")

    def attributes(**kwargs):
        if stage == "create":
            raise RuntimeError("SDK attribute creation failed")
        return FailingAttributes()

    client = CommonClient()
    with patch.object(langfuse_client, "_client", return_value=client), patch.object(langfuse_client, "_langfuse_propagate_attributes", attributes):
        with langfuse_client.span("request", user_id="user") as observation:
            assert observation is client.observation


@pytest.mark.parametrize("failure", [ValueError("application failed"), asyncio.CancelledError()])
def test_real_sdk_middleware_propagates_error_and_cancellation_and_cleans_up(real_sdk, failure):
    client, exporter = real_sdk
    original_context = context.get_current()

    async def respond(_request):
        raise failure

    with patch.object(main, "_INTERNAL_WORKER_SECRET", "test-secret"):
        with pytest.raises(type(failure)) as caught:
            asyncio.run(main.langfuse_request_tracing(request({
                "x-internal-worker-secret": "test-secret", "x-sifto-user-id": "user-a",
                "x-sifto-item-id": "item-a",
            }), respond))
    assert caught.value is failure
    assert context.get_current() == original_context
    with langfuse_client.span("after-failed-request"):
        pass
    spans = exported(client, exporter)
    assert spans["worker:extract-body"].attributes["user.id"] == "user-a"
    assert spans["worker:extract-body"].attributes["session.id"] == "item:item-a"
    assert "user.id" not in spans["after-failed-request"].attributes


def test_health_request_never_creates_observation():
    async def respond(_request):
        return Response(status_code=200)

    with patch.object(main, "langfuse_span") as start:
        response = asyncio.run(main.langfuse_request_tracing(request({}, path="/health"), respond))
    assert response.status_code == 200
    start.assert_not_called()


def test_bound_observation_updates_scores_and_prompt_references_remain_scoped():
    updates, scores = [], []
    bound = SimpleNamespace(update=lambda **kwargs: updates.append(kwargs), score=lambda **kwargs: scores.append(kwargs))

    class PromptClient(CommonClient):
        def get_prompt(self, name, *, type, label):
            assert type == "text"
            assert label == "production"
            return SimpleNamespace(version=3, compile=lambda **variables: f"Hello {variables['user']}")

    client = PromptClient()
    with patch.object(langfuse_client, "_client", return_value=client), patch.object(langfuse_client, "prompt_override_enabled", return_value=True):
        with langfuse_client.span("root") as root:
            with langfuse_client.bind_span(bound):
                assert langfuse_client.get_prompt_text("test-prompt", "fallback", variables={"user": "Sifto"}) == "Hello Sifto"
                langfuse_client.update_current(output="done")
                langfuse_client.score_current("quality", 1, comment="checked", data_type="NUMERIC")
                refs = langfuse_client._prompt_refs_var.get()
                with langfuse_client.span("child"):
                    assert langfuse_client._prompt_refs_var.get() == ()
                assert langfuse_client._prompt_refs_var.get() == refs
                assert langfuse_client._current_span_var.get() is bound
            assert langfuse_client._current_span_var.get() is root
    assert updates == [
        {"metadata": {"langfuse_prompts": [{"name": "test-prompt", "label": "production", "version": "3", "source": "langfuse"}]}},
        {"output": "done"},
    ]
    assert scores == [{"name": "quality", "value": 1, "comment": "checked", "data_type": "NUMERIC"}]
    assert langfuse_client._prompt_refs_var.get() == ()
    assert langfuse_client._current_span_var.get() is None
