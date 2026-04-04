# FUSE Smoke Test Results — 2026-04-04

## Environment
- OS: Debian (Linux 6.1.158)
- FUSE: fuse3 3.17.2, libfuse3-4, libfuse3-dev
- Runtime: Bun 1.3.11
- Node: v20.19.2

## N-API Libraries — FAILED

### fuse-native@2.2.6
- **Status:** FAILED
- **Reason:** Bundled node-gyp v6.1.0 is broken (`Cannot assign to read only property 'cflags'`)
- **Last published:** ~5 years ago
- **Verdict:** Dead library, won't build on modern Node.js

### node-fuse-bindings@2.12.4
- **Status:** FAILED
- **Reason 1:** Same bundled node-gyp v6.1.0 bug
- **Reason 2:** Even with global node-gyp v12.2.0, the C++ code targets FUSE2 API — compile errors against FUSE3 headers
- **Verdict:** FUSE2 only, incompatible with FUSE3

## FUSE3 C Binary + Bun Spawn — PASSED ✅

### Approach
1. Compile minimal FUSE3 C program against libfuse3
2. Spawn with `Bun.spawn([binary, '-f', mountpoint])`
3. Read mounted filesystem via standard Node.js `fs` API

### Results
- ✅ `gcc` compiles cleanly against FUSE3 headers
- ✅ FUSE mount works (`-f` foreground mode)
- ✅ `readdirSync()` reads directory entries from Bun
- ✅ `readFileSync()` reads file content from Bun
- ✅ ENOENT thrown for nonexistent files
- ✅ `fusermount -u` unmounts cleanly
- ✅ Process cleanup works via `proc.kill()`

### Caveat
- `/dev/fuse` required 0666 permissions (was 0600 root-only)
- Install script should check/fix this or use `allow_other` mount option
- User needs to be in `fuse` group or have `/dev/fuse` accessible

## Recommendation

**Use compiled FUSE3 helper binary.** Implementation strategy:

1. Write `src/fuse-helper.c` — full CRM filesystem (reads SQLite, serves JSON)
2. Compile during `bun build` or via install script (`gcc -o crm-fuse src/fuse-helper.c $(pkg-config --cflags --libs fuse3)`)
3. `crm mount` spawns the helper binary in foreground/daemon mode
4. `crm unmount` calls `fusermount -u`
5. Bun main process communicates mount state via PID file

**Alternative:** Bun FFI to libfuse3 directly (avoids separate binary but complex callback struct layout).

**Fallback:** `crm export-fs <dir>` — generates static file tree without FUSE. Always works, no kernel deps.
