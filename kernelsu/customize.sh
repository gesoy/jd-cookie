#!/system/bin/sh
# ============================================================
# 京东 Cookie 读取器 - customize.sh
# 同时支持 Magisk / KernelSU / APatch 安装
# ============================================================

SKIPUNZIP=0
AUTOMOUNT=true
print() { echo "- $1"; }

# 检测模块管理器 (打印信息用，实际逻辑对三者兼容)
if [ -d "/data/adb/ksu" ] || [ -f "/data/adb/ksu/manager.apk" ]; then
    MANAGER="KernelSU"
elif [ -d "/data/adb/ap" ] || [ -n "$APATCH" ]; then
    MANAGER="APatch"
else
    MANAGER="Magisk"
fi
print "检测到模块管理器: $MANAGER"

# 1. 目录权限
print "设置目录权限..."
chmod 755 "$MODPATH"
chmod 755 "$MODPATH/bin" 2>/dev/null
chmod 755 "$MODPATH/webroot" 2>/dev/null
chmod 755 "$MODPATH/system" 2>/dev/null

# 2. Go 二进制权限和 SELinux
BIN="$MODPATH/bin/jd-cookie"
if [ -f "$BIN" ]; then
    chmod 755 "$BIN"
    # 对 Magisk: 进程运行于 magisk 域，由 sepolicy.rule 放行，无需 relabel
    # 对 KernelSU: 尝试打上 su 域缺省文件上下文以兼容（失败不影响）
    chcon u:object_r:system_file:s0 "$BIN" 2>/dev/null || true
    print "jd-cookie 二进制已就绪"
else
    print "错误: 未找到 bin/jd-cookie！模块可能损坏，请重新下载"
fi

# 3. service.sh 与 sepolicy.rule 权限
[ -f "$MODPATH/service.sh" ] && chmod 755 "$MODPATH/service.sh"

if [ -f "$MODPATH/sepolicy.rule" ]; then
    chmod 644 "$MODPATH/sepolicy.rule"
    chcon u:object_r:magisk_file:s0 "$MODPATH/sepolicy.rule" 2>/dev/null || true
    print "SELinux 策略 (sepolicy.rule) 已就绪"
fi

print "安装完成！打开 WebUI 配置青龙面板，重启后服务自动运行"
