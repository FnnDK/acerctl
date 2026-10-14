#!/usr/bin/env python3
"""Watch input key events, raw scancodes, Acer HID reports, platform_profile and
the kernel log while a key is pressed.
Run as root on the host:  sudo python3 keywatch.py [seconds]
Press the key under test (and nothing else) while it runs: EV_KEY lines show the
keycode, MSC_SCAN lines the raw scancode (what a hwdb entry maps), and 'hidraw'
lines the input reports of Acer (1025) HID devices, which carry vendor events
that never become key events."""
import glob, os, select, struct, sys, time, subprocess

secs = int(sys.argv[1]) if len(sys.argv) > 1 else 15
EV_KEY, EV_MSC, MSC_SCAN = 1, 4, 4
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

raws = {}
for d in sorted(glob.glob('/sys/class/hidraw/hidraw*')):
    try:
        uevent = open(d + '/device/uevent').read()
    except OSError:
        continue
    if ':00001025:' not in uevent:
        continue
    p = '/dev/' + os.path.basename(d)
    try:
        raws[os.open(p, os.O_RDONLY | os.O_NONBLOCK)] = p
    except OSError:
        pass

pp = '/sys/class/platform-profile/platform-profile-0/profile'
def profile():
    try: return open(pp).read().strip()
    except OSError: return 'n/a'

t0 = time.time()
last = profile()
print(f'watching {len(devs)} input devices and {len(raws)} Acer hidraw nodes for {secs}s; platform_profile={last}')
print('press the key now...')
while time.time() - t0 < secs:
    r, _, _ = select.select(list(devs) + list(raws), [], [], 0.25)
    for fd in r:
        if fd in raws:
            try:
                rep = os.read(fd, 128)
            except OSError:
                continue
            print(f'  {time.time()-t0:6.2f}s  hidraw {raws[fd]}  {rep.hex(" ")}')
            continue
        try:
            data = os.read(fd, size * 64)
        except OSError:
            continue
        for off in range(0, len(data) - size + 1, size):
            sec, usec, typ, code, val = struct.unpack_from(fmt, data, off)
            p, name = devs[fd]
            if typ == EV_KEY:
                print(f'  {time.time()-t0:6.2f}s  {name!r:32} {p}  keycode={code} value={val}')
            elif typ == EV_MSC and code == MSC_SCAN:
                print(f'  {time.time()-t0:6.2f}s  {name!r:32} {p}  MSC_SCAN=0x{val & 0xffffffff:x}')
    cur = profile()
    if cur != last:
        print(f'  {time.time()-t0:6.2f}s  platform_profile changed: {last} -> {cur}')
        last = cur
print('--- kernel log (last 10 s) ---')
subprocess.run(['sh', '-c', 'dmesg | tail -n 8'])
