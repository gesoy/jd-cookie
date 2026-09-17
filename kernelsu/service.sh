#!/system/bin/sh
# ============================================================
# 京东 Cookie 读取器 - service.sh
# Magisk / KernelSU 开机 late_start 阶段运行，守护进程崩溃自动重启
# ============================================================

MODDIR="\${0%/*}"

# 读取手机时区
TZ=$(getprop persist.sys.timezone 2>/dev/null)
[ -z "$TZ" ] && TZ="Asia/Shanghai"

# 等待系统完全启动(app 数据可挂载 / 服务就绪)
until [ "$(getprop sys.boot_completed)" = "1" ]; do
    sleep 5
done

mkdir -p "$MODDIR/webroot"

run_daemon() {
    # 每次启动生成新令牌写入 webroot，供 WebUI 同源读取
    TOKEN=$(head -c16 /dev/urandom | xxd -p 2>/dev/null)
    [ -z "$TOKEN" ] && TOKEN="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
    echo "$TOKEN" > "$MODDIR/webroot/token.txt"

    "$MODDIR/bin/jd-cookie" daemon \
        --tz "$TZ" \
        --token "$TOKEN" \
        --db "$MODDIR/data.db" \
        --cookie-db "/data/data/com.jingdong.app.mall/app_webview/Default/Cookies"
}

# 守护进程崩溃/退出后 5 秒自动重启
while true; do
    run_daemon
    sleep 5
done
