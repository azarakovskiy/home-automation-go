# --- CONFIG ---
hours_needed = int(data.get("hours_needed", 6))
window_hours = int(data.get("window_hours", 12))  # for filling remaining hours
# guaranteed slots per block
guaranteed_per_block = data.get("guaranteed_per_block", {
    "night": 1,
    "morning": 1,
    "day": 2,
    "evening": 1
})

# Block definitions (hour ranges)
BLOCKS = {
    "night": list(range(23, 24)) + list(range(0, 6)),  # 23:00–05:00
    "morning": list(range(6, 12)),                     # 06:00–11:00
    "day": list(range(12, 16)),                        # 12:00–15:00
    "evening": list(range(16, 23))                     # 16:00–22:00
}

# --- CURRENT TIME ---
given_day = int(data.get("given_day", datetime.datetime.now().day))
given_hour = int(data.get("given_hour", datetime.datetime.now().hour))

try:
    local_tz = datetime.datetime.now().astimezone().tzinfo
    current_day = given_day
    current_hour = given_hour

    # --- Current hour price ---
    current_entity = hass.states.get(
        "sensor.frank_energie_prices_current_electricity_price_all_in"
    )
    if not current_entity:
        raise ValueError("Current price sensor not found")

    try:
        current_price = float(current_entity.state)
    except Exception:
        current_price = 0

    # --- Forecast ---
    forecast_entity = hass.states.get(
        "sensor.frank_energie_prices_average_electricity_price_upcoming_all_in"
    )
    forecast = forecast_entity.attributes.get("upcoming_prices", []) if forecast_entity else []

    # Build slots list: (day, hour, price, index)
    slots = []
    for i, slot in enumerate(forecast):
        iso_from = slot.get("from")
        price = float(slot.get("price", 0))
        if not iso_from:
            continue
        try:
            dt = datetime.datetime.fromisoformat(str(iso_from)).astimezone(local_tz)
            slots.append((dt.day, dt.hour, price, i))
        except Exception:
            continue

    # Add current hour explicitly at the beginning
    slots.insert(0, (current_day, current_hour, current_price, -1))

    # --- Group slots by block ---
    slots_by_block = {name: [] for name in BLOCKS}
    for d, h, price, idx in slots:
        for name, hrs in BLOCKS.items():
            if h in hrs:
                slots_by_block[name].append((d, h, price, idx))
                break

    # --- Step 1: guaranteed picks ---
    guaranteed = []
    for block_name, min_count in guaranteed_per_block.items():
        block_slots = slots_by_block.get(block_name, [])
        if not block_slots or min_count <= 0:
            continue
        # pick cheapest min_count slots in this block
        cheapest = sorted(block_slots, key=lambda x: x[2])[:min_count]
        guaranteed.extend(cheapest)

    # --- Step 2: fill remaining hours with cheapest slots across all blocks ---
    already_selected_indices = {x[3] for x in guaranteed}
    remaining_needed = max(0, hours_needed - len(guaranteed))
    remaining_slots = [x for x in slots if x[3] not in already_selected_indices]

    final_selection = list(guaranteed)

    # Fill in sliding windows of size window_hours
    remaining_slots_sorted = sorted(remaining_slots, key=lambda x: x[3])
    for win_start in range(0, len(remaining_slots_sorted), window_hours):
        if remaining_needed <= 0:
            break
        window = remaining_slots_sorted[win_start: win_start + window_hours]
        to_take = min(remaining_needed, len(window))
        cheapest_in_window = sorted(window, key=lambda x: x[2])[:to_take]
        final_selection.extend(cheapest_in_window)
        remaining_needed -= len(cheapest_in_window)

    # --- Sort final selection by original index (chronological) ---
    final_selection = sorted(final_selection, key=lambda x: x[3])

    # --- Output ---
    output["status"] = "success"
    output["hours"] = [{"day": d, "hour": h, "price": p} for d, h, p, _ in final_selection]
    output["forecast_length"] = len(slots)
    output["hours_needed"] = hours_needed
    output["window_hours"] = window_hours
    output["guaranteed_per_block"] = guaranteed_per_block
    output["given_day"] = current_day
    output["given_hour"] = current_hour

    # --- Boolean: is current hour selected? ---
    output["charge_now"] = any(d == current_day and h == current_hour for d, h, _, _ in final_selection)

except Exception as e:
    output["status"] = "error"
    output["error_message"] = str(e)
    output["hours"] = []
    output["forecast_length"] = 0
    output["charge_now"] = False