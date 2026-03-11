def wrap_json_transport(call, llm_meta_builder):
    text, usage = call()
    return text, llm_meta_builder(usage)


def wrap_anthropic_message(message, llm_meta_builder, empty_llm: dict) -> tuple[str, dict]:
    if message is None:
        return "", empty_llm
    return message.content[0].text.strip(), llm_meta_builder(message)
