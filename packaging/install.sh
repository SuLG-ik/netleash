#!/bin/bash
# SPDX-License-Identifier: GPL-3.0-or-later
# Copyright (C) 2026 Netleash contributors
#
# Installs / updates Netleash from the directory this script lives in (a release archive or dist/ after ./build.sh):
#   sudo ./install.sh            (add -y or NETLEASH_YES=1 to skip the confirmation)
# This installs a root daemon and a setuid helper. Read the README (Disclaimer, Limitations) first.
set -euo pipefail
cd "$(dirname "$0")"
[ "$(id -u)" = 0 ] || { echo "Run with sudo: sudo ./install.sh"; exit 1; }
[ -x netleash ] && [ -x netleash-exec ] || { echo "netleash binaries not found next to install.sh"; exit 1; }

USER_NAME="${SUDO_USER:-$(stat -f %Su /dev/console)}"
USER_UID=$(id -u "$USER_NAME")
N=netleash
L=/usr/local/libexec/$N
LABEL=org.netleash.daemon
AGENT_LABEL=org.netleash.app
GROUP=_netleash

echo "Netleash $(./netleash version | awk '{print $2}')"
echo "This installs a root daemon and a setuid helper (see README: Disclaimer, Limitations)."
echo "Supported setup: Claude.app + AmneziaVPN with its Kill Switch turned OFF, macOS 26, Apple Silicon."
echo "After installation, any Claude.app started outside Netleash is terminated immediately."
echo "Save your work and quit Claude first (including a Claude Code session running inside it)."
if [ "${1:-}" != "-y" ] && [ "${NETLEASH_YES:-}" != "1" ]; then
  read -r -p "Continue? [y/N] " a || a=n
  [ "$a" = y ] || { echo "Aborted."; exit 1; }
fi

# 1. Group
if ! dscl . -read /Groups/$GROUP >/dev/null 2>&1; then
  for g in $(seq 350 399); do
    dscl . -search /Groups PrimaryGroupID $g | grep -q . || { dseditgroup -o create -i $g $GROUP; break; }
  done
fi
dscl . -read /Groups/$GROUP PrimaryGroupID

# 2. Files
mkdir -p $L /usr/local/etc/$N /usr/local/bin
install -o root -g wheel -m 755 netleash $L/netleash
install -o root -g wheel -m 4755 netleash-exec $L/netleash-exec
install -o root -g wheel -m 755 panic.sh $L/panic.sh
install -o root -g wheel -m 755 resume.sh $L/resume.sh
install -o root -g wheel -m 755 uninstall.sh $L/uninstall.sh
rm -f $L/netleashd                       # binary of the pre-Go versions
ln -sf $L/netleash /usr/local/bin/netleash
[ -f /usr/local/etc/$N/config.json ] || echo '{}' > /usr/local/etc/$N/config.json
chown root:wheel /usr/local/etc/$N/config.json; chmod 644 /usr/local/etc/$N/config.json
codesign -f -s - $L/netleash $L/netleash-exec 2>/dev/null || true
chmod 4755 $L/netleash-exec               # codesign must not strip the setuid bit

# 3. Daemon (starts at boot)
launchctl enable system/$LABEL 2>/dev/null || true
cat > /Library/LaunchDaemons/$LABEL.plist <<P
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
 <key>Label</key><string>$LABEL</string>
 <key>ProgramArguments</key><array><string>$L/netleash</string><string>daemon</string></array>
 <key>RunAtLoad</key><true/>
 <key>KeepAlive</key><true/>
 <key>StandardOutPath</key><string>/var/log/$N.log</string>
 <key>StandardErrorPath</key><string>/var/log/$N.log</string>
</dict></plist>
P
chown root:wheel /Library/LaunchDaemons/$LABEL.plist; chmod 644 /Library/LaunchDaemons/$LABEL.plist
launchctl bootout system/$LABEL 2>/dev/null || true
# bootout is asynchronous: wait until the old daemon is really unloaded, otherwise bootstrap fails with "Input/output error"
for _ in $(seq 1 50); do launchctl print system/$LABEL >/dev/null 2>&1 || break; sleep 0.2; done
ok=0
for _ in $(seq 1 10); do
  if launchctl bootstrap system /Library/LaunchDaemons/$LABEL.plist 2>/dev/null; then ok=1; break; fi
  launchctl print system/$LABEL >/dev/null 2>&1 && { ok=1; break; }   # already loaded
  sleep 1
done
[ "$ok" = 1 ] || { echo "Could not start the daemon. Log: /var/log/$N.log"; exit 1; }
launchctl kickstart system/$LABEL 2>/dev/null || true

# 4. The app: launcher + menu bar icon + leak test + rollback
if [ -d Netleash.app ]; then
  launchctl asuser "$USER_UID" launchctl bootout "gui/$USER_UID/$AGENT_LABEL" 2>/dev/null || true
  pkill -u "$USER_UID" -f "Netleash.app/Contents/MacOS" 2>/dev/null || true
  APP="/Applications/Netleash.app"
  rm -rf "$APP"; cp -R Netleash.app "$APP"
  # The icon is copied from the user's own Claude.app at install time; no third-party artwork is shipped.
  cp /Applications/Claude.app/Contents/Resources/electron.icns "$APP/Contents/Resources/icon.icns" 2>/dev/null || true
  codesign -f -s - "$APP" 2>/dev/null || true
  chown -R "$USER_NAME" "$APP"

  # Start the icon at login (can be switched off in the app's menu)
  AGENT="/Users/$USER_NAME/Library/LaunchAgents/$AGENT_LABEL.plist"
  mkdir -p "$(dirname "$AGENT")"
  cat > "$AGENT" <<P
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
 <key>Label</key><string>$AGENT_LABEL</string>
 <key>ProgramArguments</key><array><string>$APP/Contents/MacOS/Netleash</string><string>--background</string></array>
 <key>RunAtLoad</key><true/>
</dict></plist>
P
  chown "$USER_NAME" "$AGENT"
  launchctl asuser "$USER_UID" launchctl bootstrap "gui/$USER_UID" "$AGENT" 2>/dev/null || echo "(the menu bar icon will appear at your next login)"
else
  echo "No menu bar app in this build: use the CLI (netleash status/test/run)."
fi

sleep 2
echo; netleash status || true
echo "Done. Open “Netleash” from /Applications (or the menu bar icon → Open Claude). Check the setup: netleash test"
