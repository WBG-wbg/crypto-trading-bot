"""
币安交易执行器 - 支持做多、做空、杠杆交易
"""
import os
import time
from typing import Dict, Any, Optional, Callable
from functools import wraps
import ccxt
from datetime import datetime


def api_retry(max_retries: int = 3, delay: float = 2.0, backoff: float = 2.0):
    """
    API 调用重试装饰器，用于处理网络错误和临时性 API 错误
    
    Args:
        max_retries: 最大重试次数
        delay: 初始延迟时间（秒）
        backoff: 延迟时间的指数退避因子
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs):
            retry_count = 0
            current_delay = delay
            
            while retry_count <= max_retries:
                try:
                    return func(*args, **kwargs)
                except (ccxt.NetworkError, ccxt.ExchangeNotAvailable, 
                        ccxt.RequestTimeout, ConnectionError) as e:
                    retry_count += 1
                    if retry_count > max_retries:
                        print(f"❌ {func.__name__} 达到最大重试次数 {max_retries}，放弃重试")
                        raise
                    
                    print(f"⚠️ {func.__name__} 网络错误 (尝试 {retry_count}/{max_retries}): {str(e)[:100]}")
                    print(f"   等待 {current_delay:.1f} 秒后重试...")
                    time.sleep(current_delay)
                    current_delay *= backoff
                    
                except Exception as e:
                    # 对于非网络错误，直接抛出
                    print(f"❌ {func.__name__} 发生非网络错误: {e}")
                    raise
            
            return func(*args, **kwargs)
        return wrapper
    return decorator


class BinanceExecutor:
    """币安期货交易执行器"""
    
    def __init__(self, config: Dict[str, Any], test_mode: bool = None):
        """
        初始化币安交易执行器
        
        Args:
            config: 配置字典
            test_mode: 是否为测试模式（不实际下单）。如果为 None，则从 config 读取
        """
        self.config = config
        # 如果没有显式指定 test_mode，从 config 读取
        if test_mode is None:
            self.test_mode = config.get('binance_test_mode', True)
        else:
            self.test_mode = test_mode
        
        # 打印当前模式（重要！）
        if self.test_mode:
            print(f"🟢 交易执行器：测试模式（模拟交易）")
        else:
            print(f"🔴 交易执行器：实盘模式（真实交易！）")
        
        # 从环境变量获取 API 密钥
        api_key = os.getenv('BINANCE_API_KEY', '')
        secret = os.getenv('BINANCE_SECRET', '')
        
        # 获取代理配置
        proxy = config.get('binance_proxy', None)
        
        exchange_config = {
            'apiKey': api_key,
            'secret': secret,
            'options': {'defaultType': 'future'},  # 期货模式
            'enableRateLimit': True,
        }
        
        if proxy:
            exchange_config['proxies'] = {
                'http': proxy,
                'https': proxy,
            }
        
        self.exchange = ccxt.binance(exchange_config)
        
        # 交易记录
        self.trade_history = []
        
        # 持仓模式（单向/双向）- 延迟检测
        self.position_mode = None  # 'hedge' (双向) 或 'oneway' (单向)
        
    def _detect_position_mode(self):
        """检测当前持仓模式"""
        if self.position_mode is not None:
            return self.position_mode
        
        # 方法0: 检查用户配置（优先级最高）
        config_mode = self.config.get('binance_position_mode', 'auto')
        if config_mode in ['oneway', 'hedge']:
            self.position_mode = config_mode
            mode_name = "单向持仓模式（One-way）" if config_mode == 'oneway' else "双向持仓模式（Hedge）"
            print(f"✓ 使用配置的持仓模式：{mode_name}")
            return self.position_mode
        
        # 自动检测模式
        try:
            # 方法1: 尝试使用ccxt的标准方法
            if hasattr(self.exchange, 'fetch_position_mode'):
                mode_info = self.exchange.fetch_position_mode()
                if mode_info.get('hedged', False):
                    self.position_mode = 'hedge'
                    print(f"✓ 检测到双向持仓模式（Hedge Mode）")
                else:
                    self.position_mode = 'oneway'
                    print(f"✓ 检测到单向持仓模式（One-way Mode）")
                return self.position_mode
        except Exception as e:
            pass  # 尝试下一个方法
        
        try:
            # 方法2: 使用币安期货API直接调用（GET /fapi/v1/positionSide/dual）
            response = self.exchange.fapi_private_get_positionside_dual()
            dual_side = response.get('dualSidePosition', False)
            
            if dual_side:
                self.position_mode = 'hedge'  # 双向持仓
                print(f"✓ 检测到双向持仓模式（Hedge Mode）")
            else:
                self.position_mode = 'oneway'  # 单向持仓
                print(f"✓ 检测到单向持仓模式（One-way Mode）")
            
            return self.position_mode
            
        except Exception as e:
            # 方法3: 默认使用单向模式（最安全）
            print(f"⚠️ 无法自动检测持仓模式")
            print(f"💡 默认使用单向持仓模式（One-way Mode）")
            print(f"💡 提示：可以在 .env 中设置 BINANCE_POSITION_MODE=oneway 或 hedge 来手动指定")
            self.position_mode = 'oneway'
            return self.position_mode
    
    def setup_exchange(self, symbol: str, leverage: int = 10):
        """
        设置交易所参数
        
        Args:
            symbol: 交易对符号
            leverage: 杠杆倍数
        """
        try:
            # 检测持仓模式
            self._detect_position_mode()
            
            # 如果需要，尝试设置为双向持仓模式（可选）
            # 注意：这会影响整个账户，可能不是所有用户都希望改变
            # if self.position_mode == 'oneway':
            #     try:
            #         self.exchange.fapiPrivatePostPositionsideDual({'dualSidePosition': 'true'})
            #         self.position_mode = 'hedge'
            #         print(f"✓ 已设置为双向持仓模式")
            #     except Exception as e:
            #         print(f"⚠️ 无法设置双向持仓模式: {e}，将使用单向模式")
            
            # 设置杠杆
            self.exchange.set_leverage(leverage, symbol)
            print(f"✓ 设置杠杆倍数: {leverage}x")
            
            # 获取余额
            balance = self.exchange.fetch_balance()
            usdt_balance = balance['USDT']['free']
            print(f"✓ 当前 USDT 余额: {usdt_balance:.2f}")
            
            return True
            
        except Exception as e:
            print(f"❌ 交易所设置失败: {e}")
            return False
    
    @api_retry(max_retries=3, delay=2.0)
    def get_current_position(self, symbol: str) -> Optional[Dict[str, Any]]:
        """
        获取当前持仓情况
        
        Args:
            symbol: 交易对符号
            
        Returns:
            持仓信息字典，如果没有持仓则返回 None
        """
        try:
            positions = self.exchange.fetch_positions([symbol])
            
            # 标准化交易对符号
            symbol_normalized = symbol if ':' in symbol else f"{symbol}:USDT"
            
            for pos in positions:
                if pos['symbol'] == symbol_normalized:
                    # 获取持仓数量
                    position_amt = 0
                    
                    if 'positionAmt' in pos.get('info', {}):
                        position_amt = float(pos['info']['positionAmt'])
                    elif 'contracts' in pos:
                        contracts = float(pos['contracts'])
                        if pos.get('side') == 'short':
                            position_amt = -contracts
                        else:
                            position_amt = contracts
                    
                    if position_amt != 0:  # 有持仓
                        side = 'long' if position_amt > 0 else 'short'
                        
                        # 安全获取价格值，处理 None 情况
                        entry_price = pos.get('entryPrice', 0)
                        entry_price = float(entry_price) if entry_price is not None else 0.0
                        
                        unrealized_pnl = pos.get('unrealizedPnl', 0)
                        unrealized_pnl = float(unrealized_pnl) if unrealized_pnl is not None else 0.0
                        
                        liquidation_price = pos.get('liquidationPrice', 0)
                        liquidation_price = float(liquidation_price) if liquidation_price is not None else 0.0
                        
                        leverage = pos.get('leverage', 1)
                        leverage = float(leverage) if leverage is not None else 1.0
                        
                        return {
                            'side': side,
                            'size': abs(position_amt),
                            'entry_price': entry_price,
                            'unrealized_pnl': unrealized_pnl,
                            'position_amt': position_amt,
                            'symbol': pos['symbol'],
                            'leverage': leverage,
                            'liquidation_price': liquidation_price
                        }
            
            return None
            
        except Exception as e:
            print(f"❌ 获取持仓失败: {e}")
            return None
    
    def execute_trade(
        self, 
        symbol: str, 
        action: str, 
        amount: float,
        stop_loss: Optional[float] = None,
        take_profit: Optional[float] = None,
        reason: str = ""
    ) -> Dict[str, Any]:
        """
        执行交易
        
        Args:
            symbol: 交易对符号，如 'BTC/USDT'
            action: 交易动作 - 'BUY' (做多), 'SELL' (做空), 'CLOSE_LONG' (平多), 'CLOSE_SHORT' (平空), 'HOLD' (持有)
            amount: 交易数量
            stop_loss: 止损价格（可选）
            take_profit: 止盈价格（可选）
            reason: 交易理由
            
        Returns:
            交易结果字典
        """
        result = {
            'success': False,
            'action': action,
            'symbol': symbol,
            'amount': amount,
            'timestamp': datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
            'reason': reason,
            'test_mode': self.test_mode
        }
        
        # 获取当前持仓
        current_position = self.get_current_position(symbol)
        
        print(f"\n{'='*60}")
        print(f"交易执行 - {action}")
        print(f"交易对: {symbol}")
        print(f"数量: {amount}")
        print(f"理由: {reason}")
        print(f"当前持仓: {current_position}")
        print(f"{'='*60}")
        
        if self.test_mode:
            print("⚠️ 测试模式 - 仅模拟交易，不实际下单")
            result['success'] = True
            result['message'] = "测试模式：模拟交易成功"
            return result
        
        # 检测持仓模式
        self._detect_position_mode()
        
        try:
            order = None
            
            if action == 'BUY':
                # 做多
                if current_position and current_position['side'] == 'short':
                    # 先平空仓
                    print("📤 平空仓...")
                    params = {'positionSide': 'SHORT'} if self.position_mode == 'hedge' else {}
                    order = self.exchange.create_market_buy_order(
                        symbol,
                        current_position['size'],
                        params
                    )
                    time.sleep(1)
                
                if not current_position or current_position['side'] != 'long':
                    # 开多仓
                    print("📈 开多仓...")
                    params = {'positionSide': 'LONG'} if self.position_mode == 'hedge' else {}
                    order = self.exchange.create_market_buy_order(
                        symbol,
                        amount,
                        params
                    )
                else:
                    print("⚠️ 已有多仓，不重复开仓")
                    print("💡 提示：如想继续持有，决策应为 HOLD；如想平仓，决策应为 CLOSE")
                    result['message'] = "已有多仓，不重复开仓（系统保护：防止意外加仓）"
                    return result
                    
            elif action == 'SELL':
                # 做空
                if current_position and current_position['side'] == 'long':
                    # 先平多仓
                    print("📤 平多仓...")
                    params = {'positionSide': 'LONG'} if self.position_mode == 'hedge' else {}
                    order = self.exchange.create_market_sell_order(
                        symbol,
                        current_position['size'],
                        params
                    )
                    time.sleep(1)
                
                if not current_position or current_position['side'] != 'short':
                    # 开空仓
                    print("📉 开空仓...")
                    params = {'positionSide': 'SHORT'} if self.position_mode == 'hedge' else {}
                    order = self.exchange.create_market_sell_order(
                        symbol,
                        amount,
                        params
                    )
                else:
                    print("⚠️ 已有空仓，不重复开仓")
                    print("💡 提示：如想继续持有，决策应为 HOLD；如想平仓，决策应为 CLOSE")
                    result['message'] = "已有空仓，不重复开仓（系统保护：防止意外加仓）"
                    return result
                    
            elif action == 'CLOSE_LONG':
                # 平多仓
                if current_position and current_position['side'] == 'long':
                    print("📤 平多仓...")
                    params = {'positionSide': 'LONG'} if self.position_mode == 'hedge' else {'reduceOnly': True}
                    order = self.exchange.create_market_sell_order(
                        symbol,
                        current_position['size'],
                        params
                    )
                else:
                    print("⚠️ 没有多仓可平")
                    result['message'] = "没有多仓可平"
                    return result
                    
            elif action == 'CLOSE_SHORT':
                # 平空仓
                if current_position and current_position['side'] == 'short':
                    print("📤 平空仓...")
                    params = {'positionSide': 'SHORT'} if self.position_mode == 'hedge' else {'reduceOnly': True}
                    order = self.exchange.create_market_buy_order(
                        symbol,
                        current_position['size'],
                        params
                    )
                else:
                    print("⚠️ 没有空仓可平")
                    result['message'] = "没有空仓可平"
                    return result
                    
            elif action == 'HOLD':
                print("💤 建议观望，不执行交易")
                result['success'] = True
                result['message'] = "观望，不执行交易"
                return result
            else:
                print(f"❌ 未知的交易动作: {action}")
                result['message'] = f"未知的交易动作: {action}"
                return result
            
            # 记录订单信息
            if order:
                result['success'] = True
                result['order_id'] = order.get('id')
                result['price'] = order.get('price')
                result['filled'] = order.get('filled')
                result['message'] = "订单执行成功"
                
                print(f"✅ 订单执行成功")
                print(f"订单ID: {order.get('id')}")
                
                # 设置止损止盈（如果提供）
                if stop_loss or take_profit:
                    time.sleep(1)
                    self._set_stop_orders(symbol, stop_loss, take_profit)
                
                # 更新持仓信息
                time.sleep(2)
                new_position = self.get_current_position(symbol)
                result['new_position'] = new_position
                print(f"更新后持仓: {new_position}")
                
                # 记录到历史
                self.trade_history.append(result.copy())
            
            return result
            
        except Exception as e:
            print(f"❌ 订单执行失败: {e}")
            import traceback
            traceback.print_exc()
            result['message'] = f"订单执行失败: {str(e)}"
            return result
    
    def _set_stop_orders(
        self, 
        symbol: str, 
        stop_loss: Optional[float] = None,
        take_profit: Optional[float] = None
    ):
        """
        设置止损止盈订单
        
        Args:
            symbol: 交易对符号
            stop_loss: 止损价格
            take_profit: 止盈价格
        """
        try:
            current_position = self.get_current_position(symbol)
            if not current_position:
                return
            
            side = current_position['side']
            size = current_position['size']
            
            # 设置止损
            if stop_loss:
                if side == 'long':
                    # 多仓止损：价格跌破止损价时卖出
                    self.exchange.create_order(
                        symbol,
                        'STOP_MARKET',
                        'sell',
                        size,
                        stop_loss,
                        {'positionSide': 'LONG', 'stopPrice': stop_loss}
                    )
                    print(f"✓ 设置多仓止损: {stop_loss}")
                else:
                    # 空仓止损：价格涨破止损价时买入
                    self.exchange.create_order(
                        symbol,
                        'STOP_MARKET',
                        'buy',
                        size,
                        stop_loss,
                        {'positionSide': 'SHORT', 'stopPrice': stop_loss}
                    )
                    print(f"✓ 设置空仓止损: {stop_loss}")
            
            # 设置止盈
            if take_profit:
                if side == 'long':
                    # 多仓止盈：价格涨到止盈价时卖出
                    self.exchange.create_order(
                        symbol,
                        'TAKE_PROFIT_MARKET',
                        'sell',
                        size,
                        take_profit,
                        {'positionSide': 'LONG', 'stopPrice': take_profit}
                    )
                    print(f"✓ 设置多仓止盈: {take_profit}")
                else:
                    # 空仓止盈：价格跌到止盈价时买入
                    self.exchange.create_order(
                        symbol,
                        'TAKE_PROFIT_MARKET',
                        'buy',
                        size,
                        take_profit,
                        {'positionSide': 'SHORT', 'stopPrice': take_profit}
                    )
                    print(f"✓ 设置空仓止盈: {take_profit}")
                    
        except Exception as e:
            print(f"⚠️ 设置止损止盈失败: {e}")
    
    @api_retry(max_retries=3, delay=2.0)
    def get_account_info(self) -> Dict[str, Any]:
        """获取账户信息"""
        try:
            balance = self.exchange.fetch_balance()
            
            return {
                'total_equity': balance['USDT']['total'],
                'available_balance': balance['USDT']['free'],
                'margin_balance': balance['USDT']['used'],
                'unrealized_pnl': balance.get('info', {}).get('totalUnrealizedProfit', 0)
            }
        except Exception as e:
            print(f"❌ 获取账户信息失败: {e}")
            return {}
    
    def get_trade_history(self) -> list:
        """获取交易历史记录"""
        return self.trade_history
    
    def close_position(self, symbol: str) -> Dict[str, Any]:
        """
        平仓当前持仓（无论多空）
        
        Args:
            symbol: 交易对符号
            
        Returns:
            平仓结果字典
        """
        try:
            position = self.get_current_position(symbol)
            
            if not position or position['side'] in ['NONE', 'none', None]:
                return {
                    "status": "info",
                    "message": "没有持仓需要平仓"
                }
            
            side = position['side'].lower()
            size = position.get('size', 0)
            
            if size == 0:
                return {
                    "status": "info",
                    "message": "持仓数量为0，无需平仓"
                }
            
            print(f"准备平仓: {side.upper()} {size} {symbol}")
            
            if self.test_mode:
                result = {
                    "status": "test",
                    "action": f"平仓 {side.upper()}",
                    "symbol": symbol,
                    "quantity": size,
                    "message": "测试模式：平仓模拟成功"
                }
                print(f"✅ 测试模式：模拟平仓成功")
            else:
                # 检测持仓模式
                self._detect_position_mode()
                
                # 实盘模式：执行平仓
                # 多头平仓 = 卖出，空头平仓 = 买入
                close_side = 'sell' if side == 'long' else 'buy'
                
                # 根据持仓模式设置参数
                if self.position_mode == 'hedge':
                    # 双向持仓模式：使用 positionSide
                    position_side = 'LONG' if side == 'long' else 'SHORT'
                    params = {'positionSide': position_side, 'reduceOnly': True}
                else:
                    # 单向持仓模式：只使用 reduceOnly
                    params = {'reduceOnly': True}
                
                order = self.exchange.create_market_order(
                    symbol,
                    close_side,
                    abs(size),
                    params=params
                )
                
                result = {
                    "status": "success",
                    "order_id": order.get('id'),
                    "action": f"平仓 {side.upper()}",
                    "symbol": symbol,
                    "quantity": size,
                    "price": order.get('price', 'market'),
                    "message": "平仓成功"
                }
                print(f"✅ 平仓成功，订单ID: {order.get('id')}")
            
            # 记录交易
            self.trade_history.append({
                "timestamp": datetime.now(),
                "action": f"close_{side}",
                "symbol": symbol,
                "quantity": size,
                "result": result
            })
            
            return result
            
        except Exception as e:
            error_msg = f"平仓失败: {str(e)}"
            print(f"❌ {error_msg}")
            return {
                "status": "error",
                "message": error_msg
            }
    
    def close_all_positions(self, symbol: str):
        """平掉所有持仓"""
        position = self.get_current_position(symbol)
        if position:
            if position['side'] == 'long':
                self.execute_trade(symbol, 'CLOSE_LONG', position['size'], reason="强制平仓")
            else:
                self.execute_trade(symbol, 'CLOSE_SHORT', position['size'], reason="强制平仓")
    
    @api_retry(max_retries=3, delay=2.0)
    def get_position_summary(self, symbol: str) -> str:
        """
        获取当前持仓的摘要信息（格式化为文本）
        
        Args:
            symbol: 交易对符号
            
        Returns:
            格式化的仓位摘要字符串
        """
        try:
            # 获取账户余额
            balance = self.exchange.fetch_balance()
            usdt_balance = balance.get('USDT', {}).get('free', 0)
            usdt_total = balance.get('USDT', {}).get('total', 0)
            
            # 获取当前持仓
            position = self.get_current_position(symbol)
            
            summary = f"**账户信息**:\n"
            summary += f"- 可用余额: {usdt_balance:.2f} USDT\n"
            summary += f"- 总余额: {usdt_total:.2f} USDT\n"
            summary += f"- 已使用保证金: {(usdt_total - usdt_balance):.2f} USDT\n\n"
            
            if position and position['side'] not in ['NONE', 'none', None]:
                side = position['side']
                side_upper = side.upper()
                side_cn = "多头" if side_upper == "LONG" else "空头"
                
                # 使用正确的字段名，处理 None 值
                size = position.get('size', 0) or 0
                entry_price = position.get('entry_price', 0) or 0
                pnl = position.get('unrealized_pnl', 0) or 0
                liquidation = position.get('liquidation_price', 0) or 0
                
                # 获取当前价格（用于计算盈亏百分比）
                try:
                    ticker = self.exchange.fetch_ticker(symbol)
                    current_price = ticker['last'] or entry_price or 0
                except:
                    current_price = entry_price or 0
                
                # 计算盈亏百分比
                if entry_price > 0:
                    if side_upper == 'LONG':
                        pnl_pct = ((current_price - entry_price) / entry_price) * 100
                    else:  # SHORT
                        pnl_pct = ((entry_price - current_price) / entry_price) * 100
                else:
                    pnl_pct = 0
                
                summary += f"**当前持仓 {symbol}**:\n"
                summary += f"- 方向: {side_cn} ({side_upper})\n"
                summary += f"- 数量: {abs(size):.4f}\n"
                summary += f"- 开仓价格: ${entry_price:.2f}\n"
                summary += f"- 当前价格: ${current_price:.2f}\n"
                summary += f"- 未实现盈亏: {pnl:+.2f} USDT ({pnl_pct:+.2f}%)\n"
                
                if liquidation > 0:
                    summary += f"- 爆仓价格: ${liquidation:.2f}\n"
                    
                    # 计算距离爆仓的距离
                    if side_upper == 'LONG':
                        liquidation_distance = ((current_price - liquidation) / current_price) * 100
                    else:
                        liquidation_distance = ((liquidation - current_price) / current_price) * 100
                    
                    if liquidation_distance < 10:
                        summary += f"  ⚠️ 距离爆仓仅 {liquidation_distance:.2f}%，风险极高！\n"
                
                # 提供建议
                if pnl_pct < -5:
                    summary += f"\n⚠️ **警告**: 当前浮亏 {pnl_pct:.2f}%，已超过 -5%，建议考虑止损\n"
                elif pnl_pct > 3:
                    summary += f"\n✅ **盈利中**: 当前浮盈 {pnl_pct:.2f}%，已超过 +3%，可考虑止盈或继续持有\n"
                else:
                    summary += f"\n📊 **状态正常**: 当前盈亏在合理范围内\n"
            else:
                summary += f"**当前持仓 {symbol}**: 无持仓\n"
                summary += f"\n💡 **建议**: 可以根据市场分析开新仓位\n"
            
            return summary
            
        except Exception as e:
            error_msg = f"**获取账户信息失败** (已重试3次): {str(e)}\n\n"
            error_msg += "可能原因：\n"
            error_msg += "  - 网络连接问题\n"
            error_msg += "  - API 限流\n"
            error_msg += "  - 交易所暂时不可用\n\n"
            error_msg += "💡 **建议**: 系统将按新开仓位处理，注意控制风险"
            return error_msg

