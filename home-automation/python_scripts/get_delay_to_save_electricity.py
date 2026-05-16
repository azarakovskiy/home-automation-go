def load_upcoming_prices():
    """Load ALL available upcoming prices - no artificial limits"""
    upcoming_prices = hass.states.get('sensor.frank_energie_prices_average_electricity_price_upcoming_all_in').attributes["upcoming_prices"]
    
    price_slots = []
    
    for slot in upcoming_prices:  # Load ALL available prices
        price_slots.append(float(slot['price']))
    
    return price_slots
    
def load_current_price(): 
    price_state = hass.states.get('sensor.frank_energie_prices_current_electricity_price_all_in').state
    return float(price_state)

def minutes_until_hour_end():
    current_time = datetime.datetime.now()
    minutes_past = current_time.minute
    seconds_past = current_time.second
    
    if seconds_past > 0:
        return 60 - minutes_past - 1
    else:
        return 60 - minutes_past if minutes_past > 0 else 60

def humanize_minutes(total_minutes: int) -> str:
    hours, minutes = divmod(total_minutes, 60)
    parts = []
    if hours > 0:
        parts.append(f"{hours}h")
    if minutes > 0 or not parts:  # show "0m" if no hours
        parts.append(f"{minutes}m")
    return " ".join(parts)
    
def calculate_weighted_cost(start_delay, future_prices, current_hour_price,
                           minutes_left_in_hour, runtime_minutes, weights):
    """Calculate weighted cost for optimization - uses consumption weights per stage."""
    stage_duration = runtime_minutes / len(weights)
    total_weighted_cost = 0
    total_weighted_time = 0
    current_minute = start_delay
    
    for weight in weights:
        remaining_duration = stage_duration
        
        while remaining_duration > 0.01:
            if current_minute < minutes_left_in_hour:
                price = current_hour_price
                time_chunk = min(remaining_duration, minutes_left_in_hour - current_minute)
            else:
                minutes_into_future = current_minute - minutes_left_in_hour
                hour_index = int(minutes_into_future // 60)
                
                if hour_index < len(future_prices):
                    price = future_prices[hour_index]
                else:
                    price = future_prices[-1]
                
                minutes_in_hour = minutes_into_future % 60
                time_chunk = min(remaining_duration, 60 - minutes_in_hour)
            
            total_weighted_cost += time_chunk * price * weight
            total_weighted_time += time_chunk * weight
            current_minute += time_chunk
            remaining_duration -= time_chunk
    
    return total_weighted_cost / total_weighted_time if total_weighted_time > 0 else 0

def calculate_simple_average_price(start_delay, future_prices, current_hour_price,
                                  minutes_left_in_hour, runtime_minutes):
    """Calculate simple time-weighted average price per kWh - NO consumption weights."""
    total_cost = 0
    total_time = 0
    current_minute = start_delay
    remaining_duration = runtime_minutes
    
    while remaining_duration > 0.01:
        if current_minute < minutes_left_in_hour:
            price = current_hour_price
            time_chunk = min(remaining_duration, minutes_left_in_hour - current_minute)
        else:
            minutes_into_future = current_minute - minutes_left_in_hour
            hour_index = int(minutes_into_future // 60)
            
            if hour_index < len(future_prices):
                price = future_prices[hour_index]
            else:
                price = future_prices[-1]
            
            minutes_in_hour = minutes_into_future % 60
            time_chunk = min(remaining_duration, 60 - minutes_in_hour)
        
        total_cost += time_chunk * price
        total_time += time_chunk
        current_minute += time_chunk
        remaining_duration -= time_chunk
    
    return total_cost / total_time if total_time > 0 else 0

def get_optimal_electricity_delay(runtime_minutes, weights=None, max_delay_hours=6):
    """
    Find optimal delay in minutes to minimize electricity cost using 5-minute slot analysis.
    """
    if weights is None:
        weights = [1]
    
    # Load ALL available price data - no limits!
    future_prices = load_upcoming_prices()
    current_hour_price = load_current_price()
    minutes_left_in_hour = minutes_until_hour_end()
    
    if not future_prices:
        return 0
    
    # Calculate maximum start delay based on finish constraint
    max_finish_time_minutes = max_delay_hours * 60 + runtime_minutes * 0.1
    max_start_delay_minutes = max_finish_time_minutes - runtime_minutes
    max_start_delay_minutes = max(0, int(max_start_delay_minutes))
    
    # Generate 5-minute slots to evaluate  
    slots = list(range(0, max_start_delay_minutes + 1, 5))
    if max_start_delay_minutes not in slots and max_start_delay_minutes > 0:
        slots.append(max_start_delay_minutes)
    
    best_cost = float('inf')
    best_slots = []
    
    # Use weighted cost for optimization
    for start_delay in slots:
        weighted_cost = calculate_weighted_cost(
            start_delay, future_prices, current_hour_price,
            minutes_left_in_hour, runtime_minutes, weights
        )
        
        if weighted_cost < best_cost:
            best_cost = weighted_cost
            best_slots = [start_delay]
        elif abs(weighted_cost - best_cost) / best_cost <= 0.02:  # 2% tolerance
            best_slots.append(start_delay)
    
    return min(best_slots) if best_slots else 0

# Main execution with enhanced tolerance logic
try:
    runtime_minutes = data.get('runtime_minutes')
    weights = data.get("weights", [1.0])
    max_delay_hours = data.get('max_delay_hours', 6)

    optimal_delay = get_optimal_electricity_delay(runtime_minutes, weights, max_delay_hours)
    
    # Calculate prices for comparison
    future_prices = load_upcoming_prices()
    current_hour_price = load_current_price()
    minutes_left_in_hour = minutes_until_hour_end()
    
    # Use simple average for price reporting (no weights)
    current_avg_price = calculate_simple_average_price(
        0, future_prices, current_hour_price,
        minutes_left_in_hour, runtime_minutes
    )
    
    optimal_avg_price = calculate_simple_average_price(
        optimal_delay, future_prices, current_hour_price,
        minutes_left_in_hour, runtime_minutes
    )
    
    # Calculate savings percentage
    if current_avg_price > 0:
        savings_percentage = ((current_avg_price - optimal_avg_price) / current_avg_price) * 100
    else:
        savings_percentage = 0

    # ENHANCED: Multi-tier tolerance system
    MINIMUM_SAVINGS_THRESHOLD = 2.0  # Basic threshold
    
    # Calculate delay efficiency (savings per hour of delay)
    delay_hours = optimal_delay / 60 if optimal_delay > 0 else 0
    delay_efficiency = savings_percentage / delay_hours if delay_hours > 0 else 0
    
    # Enhanced tolerance logic
    tolerance_reason = None
    
    if savings_percentage < MINIMUM_SAVINGS_THRESHOLD:
        # Basic threshold check
        optimal_delay = 0
        optimal_avg_price = current_avg_price
        savings_percentage = 0.0
        tolerance_reason = f"savings_{savings_percentage:.1f}%_below_{MINIMUM_SAVINGS_THRESHOLD}%"
        
    elif runtime_minutes < 120 and delay_efficiency < 0.5:
        # Short processes need better efficiency
        optimal_delay = 0
        optimal_avg_price = current_avg_price
        savings_percentage = 0.0
        tolerance_reason = f"short_process_poor_efficiency_{delay_efficiency:.2f}%_per_hour"
        
    elif runtime_minutes >= 120 and delay_efficiency < 0.3:
        # Long processes can accept lower efficiency
        optimal_delay = 0
        optimal_avg_price = current_avg_price
        savings_percentage = 0.0
        tolerance_reason = f"long_process_poor_efficiency_{delay_efficiency:.2f}%_per_hour"
        
    elif optimal_delay > 360:  # More than 6 hours
        # Practical limit: don't wait more than 6 hours for small savings
        if savings_percentage < 5.0:
            optimal_delay = 0
            optimal_avg_price = current_avg_price
            savings_percentage = 0.0
            tolerance_reason = f"long_delay_{optimal_delay}min_insufficient_savings_{savings_percentage:.1f}%"

    now = datetime.datetime.now()
    total_minutes = now.minute + optimal_delay
    hours = (now.hour + total_minutes // 60) % 24
    minutes = total_minutes % 60
    
    output["optimal_start_at_text"] = f"{hours:02d}:{minutes:02d}"
    output["optimal_delay_minutes"] = int(optimal_delay)
    output["optimal_delay_seconds"] = int(optimal_delay * 60)
    output["optimal_delay_text"] = humanize_minutes(optimal_delay)
    output["best_average_price_with_delay"] = round(optimal_avg_price, 4)
    output["current_average_price"] = round(current_avg_price, 4)
    output["cost_savings_percent"] = int(savings_percentage)
    output["status"] = "success"
    
    # Enhanced debug output
    output["debug_hours_available"] = len(future_prices)
    output["debug_first_6_future"] = future_prices[:6]
    output["debug_tolerance_applied"] = tolerance_reason is not None
    output["debug_tolerance_reason"] = tolerance_reason
    output["debug_delay_efficiency_percent_per_hour"] = round(delay_efficiency, 2)

except Exception as e:
    output["status"] = "error"
    output["error_message"] = str(e)
    output["optimal_delay_minutes"] = 0
    output["best_price"] = 0
    output["current_price"] = 0
    output["cost_savings_percent"] = 0
    output["debug_tolerance_applied"] = False
    output["debug_tolerance_reason"] = None
    output["debug_delay_efficiency_percent_per_hour"] = 0
