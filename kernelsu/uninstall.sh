#!/system/bin/sh
# ============================================================
# 京东 Cookie 读取器 - uninstall.sh
# 卸载时终止守护进程，清理运行时数据
# ============================================================

echo "- 终止 jd-cookie 守护进程..."
# 通过 SQLite 中保存的 PID 停止（失败则按名字匹配）
PIDS=$(cat "$MODDIR/data.db" 2>/dev/null || true)

# 匹配进程名停止
for pid in $(pgrep -f 'bin/jd-cookie' 2>/dev/null); do
    kill "$pid" 2>/dev/null
done
sleep 1

echo "- 清理运行时文件..."
rm -f "$MODDIR/token.txt"
rm -rf "$MODDIR/webroot" 2>/dev/null
rm -rf "$MODDIR/logs" 2>/dev/null

echo "- 卸载完成"

