#!/usr/bin/env python3
"""Watch input key events, platform_profile and kernel log while a key is pressed.
Run as root on the host:  sudo python3 keywatch.py [seconds]
Press the Nitro key (and nothing else) while it runs."""
import glob, os, select, struct, sys, time, subprocess

secs = int(sys.argv[1]) if len(sys.argv) > 1 else 15
EV_KEY = 1
fmt = 'llHHi'  # struct input_event on x86_64
size = struct.calcsize(fmt)

def devname(path):
    n = os.path.basename(path)
    try:
        return open(f'/sys/class/input/{n}/device/name').read().strip()
    except OSError:
        return '?'

devs = {}
for p in sorted(glob.glob('/dev/input/event*')):
    try:
        fd = os.open(p, os.O_RDONLY | os.O_NONBLOCK)
    except OSError:
        continue
    devs[fd] = (p, devname(p))

pp = '/sys/class/platform-profile/platform-profile-0/profile'
def profile():
    try: return open(pp).read().strip()
    except OSError: return 'n/a'

t0 = time.time()
last = profile()
print(f'watching {len(devs)} input devices for {secs}s; platform_profile={last}')
print('press the Nitro key now...')
while time.time() - t0 < secs:
    r, _, _ = select.select(list(devs), [], [], 0.25)
    for fd in r:
        try:
            data = os.read(fd, size * 64)
        except OSError:
            continue
        for off in range(0, len(data) - size + 1, size):
            sec, usec, typ, code, val = struct.unpack_from(fmt, data, off)
            if typ == EV_KEY:
                p, name = devs[fd]
                print(f'  {time.time()-t0:6.2f}s  {name!r:32} {p}  keycode={code} value={val}')
    cur = profile()
    if cur != last:
        print(f'  {time.time()-t0:6.2f}s  platform_profile changed: {last} -> {cur}')
        last = cur
print('--- kernel log (last 10 s) ---')
subprocess.run(['sh', '-c', 'dmesg | tail -n 8'])
