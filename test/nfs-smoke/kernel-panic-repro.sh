#!/bin/bash
# Reproduces a macOS kernel panic caused by Bun's writeFileSync on NFS mounts.
#
# writeFileSync uses O_TRUNC which triggers SETATTR(size=0) in the NFS client.
# After ~30+ such operations, the kernel's VFS vnode state corrupts and panics.
#
# Shell echo/printf uses a simpler syscall path and does NOT trigger the bug.
#
# Usage:
#   ./test/nfs-smoke/kernel-panic-repro.sh          # default 50 iterations
#   ./test/nfs-smoke/kernel-panic-repro.sh 100       # custom iteration count
#
# WARNING: This WILL kernel panic your Mac if the bug is present.
# Only run this when you're actively working on a fix.

set -e

ITERATIONS=${1:-50}
PROJECT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
CRM_NFS="$HOME/.crm/bin/crm-nfs"

if [ ! -x "$CRM_NFS" ]; then
  echo "Error: crm-nfs not found at $CRM_NFS"
  echo "Build it: cd $PROJECT_DIR/src/nfs-server && cargo build --release && cat target/release/crm-nfs > $CRM_NFS && chmod +x $CRM_NFS"
  exit 1
fi

DB=$(mktemp -u).db
SOCK=$(mktemp -u).sock
MNT=$(mktemp -d)

cleanup() {
  umount "$MNT" 2>/dev/null || true
  kill $NFS_PID $DAEMON_PID 2>/dev/null || true
  rm -rf "$MNT" "$SOCK" "$DB" "${DB}-shm" "${DB}-wal"
}
trap cleanup EXIT

echo "Starting daemon..."
bun run "$PROJECT_DIR/src/fuse-daemon.ts" "$SOCK" "$DB" 2>/dev/null &
DAEMON_PID=$!
sleep 1

echo "Starting NFS server..."
PORT=11500
"$CRM_NFS" "$SOCK" "$PORT" 2>/dev/null &
NFS_PID=$!
sleep 1

echo "Mounting..."
/sbin/mount_nfs -o locallocks,vers=3,tcp,port=$PORT,mountport=$PORT,soft,intr,timeo=10,retrans=3,noac 127.0.0.1:/ "$MNT"

echo "Running $ITERATIONS writeFileSync iterations (O_TRUNC)..."
echo "If this kernel panics, the bug is still present."
echo ""

bun -e "
const fs = require('fs');
const path = require('path');
const mp = '$MNT';
const n = $ITERATIONS;

for (let i = 0; i < n; i++) {
  const file = path.join(mp, 'contacts', 'test-' + i + '.json');
  // Create (triggers CREATE)
  fs.writeFileSync(file, JSON.stringify({name: 'Test' + i, emails: ['t' + i + '@x.com']}));
  // Overwrite (triggers SETATTR with O_TRUNC — this is what panics)
  fs.writeFileSync(file, JSON.stringify({name: 'Test' + i + 'v2', emails: ['t' + i + '@x.com']}));
  process.stdout.write('\r  ' + (i + 1) + '/' + n);
}
console.log('');
console.log('All iterations completed without kernel panic.');
"

echo "PASS: macOS survived $ITERATIONS O_TRUNC write cycles on NFS."
