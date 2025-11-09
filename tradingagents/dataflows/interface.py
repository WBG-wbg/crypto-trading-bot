from typing import Annotated

# Import crypto-specific modules only
from .crypto_ccxt import (
    get_crypto_ohlcv,
    get_crypto_indicators,
    get_funding_rate,
    get_order_book,
    get_market_info
)

# Configuration and routing logic
from .config import get_config

# Tools organized by category (simplified for crypto only)
TOOLS_CATEGORIES = {
    "crypto_data": {
        "description": "Cryptocurrency-specific data",
        "tools": [
            "get_crypto_data",
            "get_crypto_indicators",
            "get_crypto_funding_rate",
            "get_crypto_order_book",
            "get_crypto_market_info"
        ]
    }
}

VENDOR_LIST = [
    "ccxt"  # For crypto data
]

# Mapping of methods to their vendor-specific implementations (crypto only)
VENDOR_METHODS = {
    # crypto_data
    "get_crypto_data": {
        "ccxt": get_crypto_ohlcv,
    },
    "get_crypto_indicators": {
        "ccxt": get_crypto_indicators,
    },
    "get_crypto_funding_rate": {
        "ccxt": get_funding_rate,
    },
    "get_crypto_order_book": {
        "ccxt": get_order_book,
    },
    "get_crypto_market_info": {
        "ccxt": get_market_info,
    },
}

def get_category_for_method(method: str) -> str:
    """Get the category that contains the specified method."""
    for category, info in TOOLS_CATEGORIES.items():
        if method in info["tools"]:
            return category
    raise ValueError(f"Method '{method}' not found in any category")

def get_vendor(category: str, method: str = None) -> str:
    """Get the configured vendor for a data category or specific tool method.
    Tool-level configuration takes precedence over category-level.
    """
    config = get_config()

    # Check tool-level configuration first (if method provided)
    if method:
        tool_vendors = config.get("tool_vendors", {})
        if method in tool_vendors:
            return tool_vendors[method]

    # Fall back to category-level configuration
    return config.get("data_vendors", {}).get(category, "ccxt")

def route_to_vendor(method: str, *args, **kwargs):
    """Route method calls to appropriate vendor implementation (simplified for crypto)."""
    category = get_category_for_method(method)
    vendor_config = get_vendor(category, method)

    # Handle comma-separated vendors
    primary_vendors = [v.strip() for v in vendor_config.split(',')]

    if method not in VENDOR_METHODS:
        raise ValueError(f"Method '{method}' not supported")

    # Get all available vendors for this method
    all_available_vendors = list(VENDOR_METHODS[method].keys())

    # Create fallback vendor list
    fallback_vendors = primary_vendors.copy()
    for vendor in all_available_vendors:
        if vendor not in fallback_vendors:
            fallback_vendors.append(vendor)

    # Track results
    results = []
    vendor_attempt_count = 0
    successful_vendor = None

    for vendor in fallback_vendors:
        if vendor not in VENDOR_METHODS[method]:
            if vendor in primary_vendors:
                print(f"INFO: Vendor '{vendor}' not supported for method '{method}', falling back to next vendor")
            continue

        vendor_impl = VENDOR_METHODS[method][vendor]
        is_primary_vendor = vendor in primary_vendors
        vendor_attempt_count += 1

        try:
            result = vendor_impl(*args, **kwargs)
            results.append(result)
            successful_vendor = vendor
            break  # Stop after first successful vendor
        except Exception as e:
            print(f"FAILED: Vendor '{vendor}' failed for method '{method}': {e}")
            continue

    # Final result check
    if not results:
        raise RuntimeError(f"All vendor implementations failed for method '{method}'")

    # Return single result
    return results[0] if len(results) == 1 else '\n'.join(str(result) for result in results)


def route_tool_call(method: str, **kwargs):
    """
    Route tool calls to appropriate vendor implementation.
    This is a simplified version of route_to_vendor specifically for @tool decorated functions.

    Args:
        method: The method name (e.g., 'get_crypto_data')
        **kwargs: Keyword arguments to pass to the method

    Returns:
        The result from the vendor implementation
    """
    return route_to_vendor(method, **kwargs)
