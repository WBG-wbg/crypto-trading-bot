from langchain_core.messages import HumanMessage, RemoveMessage

# Import crypto-specific tools only
from tradingagents.agents.utils.crypto_tools import (
    get_crypto_funding_rate,
    get_crypto_order_book,
    get_crypto_market_info,
    get_crypto_data,
    get_crypto_indicators
)

def create_msg_delete():
    def delete_messages(state):
        """Clear messages and add placeholder for Anthropic compatibility"""
        messages = state["messages"]
        
        # Remove all messages
        removal_operations = [RemoveMessage(id=m.id) for m in messages]
        
        # Add a minimal placeholder message
        placeholder = HumanMessage(content="Continue")
        
        return {"messages": removal_operations + [placeholder]}
    
    return delete_messages


        