"""
简化版加密货币交易系统入口
只使用 3 个核心 Agent：市场分析师 + 加密货币分析师 + 交易员

支持智能定时运行：
- 15m 周期：在 0, 15, 30, 45 分钟运行
- 1h 周期：在整点运行
- 其他周期：自动对齐
"""
from tradingagents.graph.simple_crypto_graph import SimpleCryptoTradingGraph
from tradingagents.crypto_config import get_crypto_config
from tradingagents.utils.logger import ColorLogger
from tradingagents.utils.scheduler import TradingScheduler
from dotenv import load_dotenv
from datetime import datetime
import sys
import os
import json
from pathlib import Path

load_dotenv()

# Web监控相关
monitor = None
web_enabled = False


def save_results(config, symbol, trade_date, final_state, decision, execution_result=None):
    """保存分析结果到文件"""
    try:
        # 创建结果目录
        results_dir = Path(config['results_dir'])
        symbol_clean = symbol.replace('/', '_')
        symbol_dir = results_dir / symbol_clean
        symbol_dir.mkdir(parents=True, exist_ok=True)
        
        # 生成文件名（包含时间戳）
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        filename = f"{symbol_clean}_{trade_date}_{timestamp}"
        
        # 提取最后的状态
        last_state = list(final_state.values())[0] if final_state else {}
        
        # 解析决策类型
        decision_type = 'UNKNOWN'
        if '**最终决策: BUY**' in decision or '**最终决策: LONG**' in decision:
            decision_type = 'BUY'
        elif '**最终决策: SELL**' in decision or '**最终决策: SHORT**' in decision:
            decision_type = 'SELL'
        elif '**最终决策: CLOSE**' in decision:
            decision_type = 'CLOSE'
        elif '**最终决策: HOLD**' in decision:
            decision_type = 'HOLD'
        
        # 构建结果数据
        result_data = {
            'timestamp': datetime.now().isoformat(),
            'trade_date': trade_date,
            'symbol': symbol,
            'timeframe': config.get('crypto_timeframe', '1h'),
            'decision_type': decision_type,
            'decision_content': decision,
            'market_analysis': last_state.get('market_report', ''),
            'crypto_analysis': last_state.get('crypto_analysis_report', ''),
            'sentiment_analysis': last_state.get('sentiment_report', ''),
            'execution_result': execution_result,
            'config': {
                'leverage': config.get('binance_leverage'),
                'position_size': config.get('position_size'),
                'test_mode': config.get('binance_test_mode'),
                'auto_execute': config.get('auto_execute')
            }
        }
        
        # 保存为 JSON 文件
        json_file = symbol_dir / f"{filename}.json"
        with open(json_file, 'w', encoding='utf-8') as f:
            json.dump(result_data, f, ensure_ascii=False, indent=2)
        
        # 保存为纯文本文件（便于阅读）
        txt_file = symbol_dir / f"{filename}.txt"
        with open(txt_file, 'w', encoding='utf-8') as f:
            f.write(f"{'='*80}\n")
            f.write(f"交易分析报告 - {symbol}\n")
            f.write(f"{'='*80}\n")
            f.write(f"日期: {trade_date}\n")
            f.write(f"时间戳: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
            f.write(f"K线周期: {config.get('crypto_timeframe', '1h')}\n")
            f.write(f"杠杆倍数: {config.get('binance_leverage')}x\n")
            f.write(f"测试模式: {'是' if config.get('binance_test_mode') else '否'}\n")
            f.write(f"自动执行: {'是' if config.get('auto_execute') else '否'}\n")
            f.write(f"{'='*80}\n\n")
            
            f.write(f"{'─'*80}\n")
            f.write(f"📊 市场技术分析\n")
            f.write(f"{'─'*80}\n")
            f.write(last_state.get('market_report', '无数据') + "\n\n")
            
            f.write(f"{'─'*80}\n")
            f.write(f"💰 加密货币专属分析\n")
            f.write(f"{'─'*80}\n")
            f.write(last_state.get('crypto_analysis_report', '无数据') + "\n\n")
            
            f.write(f"{'─'*80}\n")
            f.write(f"🎭 市场情绪分析\n")
            f.write(f"{'─'*80}\n")
            f.write(last_state.get('sentiment_report', '无数据') + "\n\n")
            
            f.write(f"{'='*80}\n")
            f.write(f"📈 最终交易决策\n")
            f.write(f"{'='*80}\n")
            f.write(f"决策类型: {decision_type}\n\n")
            f.write(decision + "\n\n")
            
            if execution_result:
                f.write(f"{'─'*80}\n")
                f.write(f"⚡ 执行结果\n")
                f.write(f"{'─'*80}\n")
                f.write(str(execution_result) + "\n")
        
        ColorLogger.success(f"结果已保存到:")
        print(f"  📄 JSON: {json_file}")
        print(f"  📝 文本: {txt_file}")
        
        return json_file, txt_file
        
    except Exception as e:
        ColorLogger.error(f"保存结果失败: {e}")
        return None, None


def run_analysis(config):
    """执行一次完整的交易分析"""
    global monitor
    
    # 创建交易图（默认开启 debug 模式以显示详细日志）
    ta = SimpleCryptoTradingGraph(
        debug=True,  # 强制开启详细日志
        config=config,
        auto_execute=config['auto_execute']
    )
    
    # 获取当前日期
    trade_date = datetime.now().strftime("%Y-%m-%d")
    
    ColorLogger.info(f"📈 开始分析 {config['crypto_symbol']} (日期: {trade_date})")
    
    # 运行分析
    final_state, decision = ta.propagate(config['crypto_symbol'], trade_date)
    
    # 提取信息用于Web监控
    if monitor and web_enabled:
        try:
            last_state = list(final_state.values())[0] if final_state else {}
            
            # 解析决策
            decision_type = 'UNKNOWN'
            if '**最终决策: BUY**' in decision or '**最终决策: LONG**' in decision:
                decision_type = 'BUY'
            elif '**最终决策: SELL**' in decision or '**最终决策: SHORT**' in decision:
                decision_type = 'SELL'
            elif '**最终决策: CLOSE**' in decision:
                decision_type = 'CLOSE'
            elif '**最终决策: HOLD**' in decision:
                decision_type = 'HOLD'
            
            # 获取持仓信息
            position_info = ta._get_position_info(config['crypto_symbol']) if hasattr(ta, '_get_position_info') else '无法获取'
            
            # 添加到监控
            log_entry = {
                'decision': decision_type,
                'decision_content': decision,
                'market_analysis': last_state.get('market_report', ''),
                'crypto_analysis': last_state.get('crypto_analysis_report', ''),
                'sentiment_analysis': last_state.get('sentiment_report', ''),
                'position_info': position_info,
                'execution_result': None
            }
            monitor.add_log(log_entry)
        except Exception as e:
            print(f"⚠️ 添加Web监控日志失败: {e}")
    
    # 显示最终决策
    ColorLogger.decision(decision)
    
    # 自动执行交易（如果启用）
    execution_result = None
    if config['auto_execute']:
        ColorLogger.warning("自动执行模式已开启，正在执行交易...")
        execution_result = ta.execute_trade(decision)
        
        # 更新执行结果到监控
        if monitor and web_enabled and monitor.trading_logs:
            monitor.trading_logs[0]['execution_result'] = str(execution_result)
    else:
        ColorLogger.info("自动执行模式已关闭，请手动审核交易决策")
    
    # 保存结果到文件
    print()  # 空一行
    save_results(config, config['crypto_symbol'], trade_date, final_state, decision, execution_result)
    
    print(f"\n{ColorLogger.BRIGHT_GREEN}{'='*80}{ColorLogger.RESET}")
    print(f"{ColorLogger.BOLD}{ColorLogger.BRIGHT_GREEN}🎉 本次分析完成！{ColorLogger.RESET}")
    print(f"{ColorLogger.BRIGHT_GREEN}{'='*80}{ColorLogger.RESET}\n")


def main():
    """主函数 - 支持单次运行和定时循环运行"""
    # 加载配置
    config = get_crypto_config()
    
    # 使用彩色输出
    ColorLogger.header("简化版加密货币交易系统", '=', 80)
    
    print(f"{ColorLogger.BOLD}交易对:{ColorLogger.RESET} {ColorLogger.BRIGHT_YELLOW}{config['crypto_symbol']}{ColorLogger.RESET}")
    print(f"{ColorLogger.BOLD}K线周期:{ColorLogger.RESET} {config['crypto_timeframe']}")
    print(f"{ColorLogger.BOLD}LLM 模型:{ColorLogger.RESET} {config['quick_think_llm']}")
    print(f"{ColorLogger.BOLD}杠杆倍数:{ColorLogger.RESET} {ColorLogger.BRIGHT_RED}{config['binance_leverage']}x{ColorLogger.RESET}")
    
    if config['binance_test_mode']:
        print(f"{ColorLogger.BOLD}测试模式:{ColorLogger.RESET} {ColorLogger.GREEN}是 ✅{ColorLogger.RESET}")
    else:
        print(f"{ColorLogger.BOLD}测试模式:{ColorLogger.RESET} {ColorLogger.BRIGHT_RED}否 ⚠️（实盘）{ColorLogger.RESET}")
    
    if config['binance_proxy']:
        print(f"{ColorLogger.BOLD}代理设置:{ColorLogger.RESET} {config['binance_proxy']}")
    
    if config['auto_execute']:
        print(f"{ColorLogger.BOLD}自动执行:{ColorLogger.RESET} {ColorLogger.BRIGHT_GREEN}已启用 🤖{ColorLogger.RESET}")
    else:
        print(f"{ColorLogger.BOLD}自动执行:{ColorLogger.RESET} {ColorLogger.YELLOW}未启用 👀{ColorLogger.RESET}")
    
    print(f"\n{ColorLogger.CYAN}{'─' * 80}{ColorLogger.RESET}")
    print(f"{ColorLogger.BOLD}{ColorLogger.CYAN}📊 工作流程:{ColorLogger.RESET}")
    print(f"{ColorLogger.CYAN}   1️⃣  市场分析师 → 技术指标分析{ColorLogger.RESET}")
    print(f"{ColorLogger.CYAN}   2️⃣  加密货币分析师 → 资金费率、订单簿分析{ColorLogger.RESET}")
    print(f"{ColorLogger.CYAN}   3️⃣  市场情绪分析师 → CryptoOracle 情绪指标{ColorLogger.RESET}")
    print(f"{ColorLogger.CYAN}   4️⃣  交易员 → 综合决策{ColorLogger.RESET}")
    print(f"{ColorLogger.CYAN}{'─' * 80}{ColorLogger.RESET}\n")
    
    # 检查命令行参数，判断是单次运行还是循环运行
    global monitor, web_enabled
    run_mode = 'once'  # 默认单次运行
    web_port = 5000
    
    if len(sys.argv) > 1:
        if sys.argv[1] == '--loop' or sys.argv[1] == '-l':
            run_mode = 'loop'
            # 检查是否启用Web监控
            if '--web' in sys.argv or '-w' in sys.argv:
                web_enabled = True
                # 检查是否指定端口
                try:
                    port_idx = sys.argv.index('--port') + 1 if '--port' in sys.argv else sys.argv.index('-p') + 1 if '-p' in sys.argv else None
                    if port_idx and port_idx < len(sys.argv):
                        web_port = int(sys.argv[port_idx])
                except:
                    pass
        elif sys.argv[1] == '--now' or sys.argv[1] == '-n':
            run_mode = 'once'
        elif sys.argv[1] == '--help' or sys.argv[1] == '-h':
            print("使用方法:")
            print("  python main_simple_crypto.py                    # 单次运行（立即执行）")
            print("  python main_simple_crypto.py --now              # 单次运行（立即执行）")
            print("  python main_simple_crypto.py --loop             # 循环运行（按K线周期定时）")
            print("  python main_simple_crypto.py --loop --web       # 循环运行 + Web监控")
            print("  python main_simple_crypto.py --loop --web --port 8080  # 自定义端口")
            print()
            return
    
    # 启动Web监控（如果启用）
    if web_enabled:
        try:
            from tradingagents.web.monitor import monitor as web_monitor, start_monitor_thread
            monitor = web_monitor
            start_monitor_thread(host='0.0.0.0', port=web_port)
            ColorLogger.success(f"🌐 Web监控已启动")
            print(f"{ColorLogger.CYAN}   访问地址: http://localhost:{web_port}{ColorLogger.RESET}")
            print(f"{ColorLogger.CYAN}   或使用: http://你的IP:{web_port}{ColorLogger.RESET}\n")
            
            # 更新初始状态
            monitor.update_status({
                'running': True,
                'run_count': 0,
                'next_run': None
            })
        except Exception as e:
            ColorLogger.error(f"Web监控启动失败: {e}")
            ColorLogger.warning("将继续运行但不提供Web监控")
            web_enabled = False
    
    # 显示运行模式
    timeframe = config['crypto_timeframe']
    if run_mode == 'loop':
        ColorLogger.info(f"🔄 循环模式：将在每个 {timeframe} K线周期结束时自动运行")
        intervals = TradingScheduler.get_aligned_intervals(timeframe)
        print(f"{ColorLogger.CYAN}   每天运行时间点: {', '.join(intervals[:8])}... (共 {len(intervals)} 次){ColorLogger.RESET}")
        print(f"{ColorLogger.YELLOW}   按 Ctrl+C 可随时停止{ColorLogger.RESET}\n")
    else:
        ColorLogger.info(f"▶️  单次模式：立即执行一次分析")
        print()
    
    # 运行逻辑
    if run_mode == 'once':
        # 单次运行
        run_analysis(config)
    else:
        # 循环运行
        run_count = 0
        try:
            while True:
                run_count += 1
                
                # 等待到下一个K线周期
                if run_count > 1:  # 第一次不等待，立即运行
                    print(f"\n{ColorLogger.CYAN}{'='*80}{ColorLogger.RESET}")
                    ColorLogger.info(f"等待下一个 {timeframe} K线周期...")
                    print(f"{ColorLogger.CYAN}{'='*80}{ColorLogger.RESET}\n")
                    TradingScheduler.wait_for_next_timeframe(timeframe, verbose=True)
                
                # 显示运行次数
                ColorLogger.header(f"第 {run_count} 次分析", '=', 80)
                
                # 更新Web监控状态
                if monitor and web_enabled:
                    next_time = TradingScheduler.get_next_timeframe_time(timeframe)
                    monitor.update_status({
                        'running': True,
                        'run_count': run_count,
                        'next_run': next_time.strftime('%Y-%m-%d %H:%M:%S')
                    })
                
                # 执行分析
                run_analysis(config)
                
                # 显示下次运行时间
                next_time = TradingScheduler.get_next_timeframe_time(timeframe)
                print(f"\n{ColorLogger.CYAN}{'─'*80}{ColorLogger.RESET}")
                ColorLogger.info(f"下次运行时间: {next_time.strftime('%Y-%m-%d %H:%M:%S')}")
                print(f"{ColorLogger.CYAN}{'─'*80}{ColorLogger.RESET}\n")
                
        except KeyboardInterrupt:
            print(f"\n\n{ColorLogger.YELLOW}{'='*80}{ColorLogger.RESET}")
            ColorLogger.warning("用户中断")
            print(f"{ColorLogger.YELLOW}{'='*80}{ColorLogger.RESET}")
            print(f"\n{ColorLogger.CYAN}总共运行了 {run_count} 次分析{ColorLogger.RESET}")
            print(f"{ColorLogger.CYAN}感谢使用！{ColorLogger.RESET}\n")


if __name__ == "__main__":
    main()

