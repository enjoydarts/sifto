from app.services.langfuse_client import update_current


def observe_request_input(metadata: dict | None = None, input_payload: dict | None = None) -> None:
    update_current(metadata=metadata or {}, input=input_payload or {})


def observe_request_output(output_payload: dict | None = None) -> None:
    update_current(output=output_payload or {})
