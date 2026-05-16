# Arguments passed from HA:
#   entity_id: required, the input_select entity
#   options: list of options to toggle between (at least 2)

entity_id = data.get("entity_id")
options = data.get("options")

if not entity_id or not options or len(options) < 2:
    logger.error("toggle_select: missing or invalid arguments")
else:
    current = hass.states.get(entity_id).state

    if current not in options:
        # fallback to the first option
        next_option = options[0]
    else:
        idx = options.index(current)
        next_option = options[(idx + 1) % len(options)]

    hass.services.call(
        "input_select",
        "select_option",
        {"entity_id": entity_id, "option": next_option},
        False
    )

    logger.info(
        f"toggle_select: {entity_id} switched {current!r} → {next_option!r}"
    )
