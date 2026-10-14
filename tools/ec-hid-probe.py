#!/usr/bin/env python3
"""Read-only queries to the EC's vendor HID channel (report 0xA0 of 1025:174B).

Newer non-gaming Acer laptops (e.g. Aspire AG15-71P) expose operating modes and
battery settings through the EC's HID-over-I2C device 1025:174B instead of WMI.
Commands go out as a 65-byte feature report (report ID 0xA0) and the answer is
read back with a get-feature on the same report ID:

  request:  A0 | 00 A0 | id(lo) id(hi) | sub | args...
  response: A0 | status(lo) status(hi) (0xE000 = OK) | id echo (2 bytes) | data...

Only query sub-functions are sent: everything that sets a value (sub 0x01) is
refused. The battery serial number in the battery information answer is masked.
Run as root on the host:

  sudo python3 -I tools/ec-hid-probe.py             # all queries
  sudo python3 -I tools/ec-hid-probe.py -n          # print requests, touch nothing
  sudo python3 -I tools/ec-hid-probe.py -o out.txt  # also save the output
"""
import argparse, fcntl, glob, os, sys, time

VID, PID = 0x1025, 0x174B
REPORT_ID = 0xA0
REPORT_LEN = 65      # report ID + 64 bytes
STATUS_OK = 0xE000

# (label, command id, sub-function, args).
QUERIES = [
    ('HID spec version',             0x0000, 0x00, []),
    ('battery boost status',         0x0000, 0x02, [0, 0, 0]),
    ('AC status',                    0x0000, 0x03, [0, 0, 0]),
    ('OC profile capability',        0x0000, 0x04, [0, 0, 0]),
    ('system usage mode capability', 0x0000, 0x05, [0, 0, 0]),
    ('system usage mode limit',      0x0000, 0x06, [0, 0, 0]),
    ('PD adaptor capability',        0x0000, 0x07, [0, 0, 0]),
    ('current system usage mode',    0x0001, 0x02, [0, 0, 0]),
    ('fan table capability',         0x0004, 0x02, [0, 0, 0]),
    ('fan control status',           0x0004, 0x02, [0, 0, 2]),
    # The last argument (battery index?) is not confirmed.
    ('battery information',          0x0B00, 0x02, [0, 0, 0]),
    ('battery boundary',             0x0B03, 0x02, [0, 0, 0]),
]

# Modes offered at each capability level (byte 5 of the capability answer).
# The byte each mode takes in the set command is not confirmed yet.
MODE_SETS = {
    0: 'Normal', 1: 'Normal', 2: 'Normal',
    3: 'Silent, Normal, Performance',
    4: 'Silent, Normal, Performance, Turbo',
    5: 'Eco, Silent, Normal, Performance, Turbo',
}


def ioc(nr, size):
    # _IOC(_IOC_READ | _IOC_WRITE, 'H', nr, size)
    return (3 << 30) | (size << 16) | (ord('H') << 8) | nr


HIDIOCSFEATURE = ioc(0x06, REPORT_LEN)
HIDIOCGFEATURE = ioc(0x07, REPORT_LEN)


def find_device():
    want = 'HID_ID=0018:%08X:%08X' % (VID, PID)
    for d in sorted(glob.glob('/sys/class/hidraw/hidraw*')):
        try:
            uevent = open(d + '/device/uevent').read()
            desc = open(d + '/device/report_descriptor', 'rb').read()
        except OSError:
            continue
        if want in uevent.upper().split('\n') and b'\x85' + bytes([REPORT_ID]) in desc:
            return '/dev/' + os.path.basename(d)
    return None


def request(cmd_id, sub, args):
    if sub == 0x01:
        raise ValueError('set sub-functions are not allowed')
    buf = bytearray(REPORT_LEN)
    body = [REPORT_ID, 0x00, 0xA0, cmd_id & 0xFF, cmd_id >> 8, sub] + args
    buf[:len(body)] = bytes(body)
    return buf


def hexs(b):
    return ' '.join('%02x' % x for x in b)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split('\n')[0])
    ap.add_argument('-d', '--device', help='hidraw node (default: autodetect %04X:%04X)' % (VID, PID))
    ap.add_argument('-n', '--dry-run', action='store_true', help='print the requests only')
    ap.add_argument('-o', '--output', help='also write the output to this file')
    opt = ap.parse_args()

    lines = []
    def out(s=''):
        print(s)
        lines.append(s)

    dev = opt.device or find_device()
    out('acerctl ec-hid-probe, %s' % time.strftime('%Y-%m-%d %H:%M:%S'))
    try:
        out('model: %s' % open('/sys/class/dmi/id/product_name').read().strip())
    except OSError:
        pass
    out('device: %s' % (dev or 'not found'))
    if not dev and not opt.dry_run:
        out('no hidraw node for %04X:%04X with report 0x%02X' % (VID, PID, REPORT_ID))
        return 1

    fd = None if opt.dry_run else os.open(dev, os.O_RDWR)
    answers = {}
    try:
        for label, cmd_id, sub, args in QUERIES:
            req = request(cmd_id, sub, args)
            out('\n### %s (id 0x%04X, sub 0x%02X)' % (label, cmd_id, sub))
            out('  >> %s' % hexs(req[:12]))
            if fd is None:
                continue
            try:
                fcntl.ioctl(fd, HIDIOCSFEATURE, bytes(req))
                rsp = bytearray(REPORT_LEN)
                rsp[0] = REPORT_ID
                fcntl.ioctl(fd, HIDIOCGFEATURE, rsp)
            except OSError as e:
                out('  error: %s' % e)
                continue
            status = rsp[1] | rsp[2] << 8
            echo = rsp[3] | rsp[4] << 8
            if cmd_id == 0x0B00:
                rsp[32:34] = b'\0\0'   # battery serial number, kept out of reports
            for i in range(0, REPORT_LEN, 24):
                out('  %s %s' % ('<<' if i == 0 else '  ', hexs(rsp[i:i + 24])))
            out('  status 0x%04X%s, id echo 0x%04X%s, data[5..]: %s' % (
                status, ' (OK)' if status == STATUS_OK else '',
                echo, '' if echo == cmd_id else ' (mismatch)', hexs(rsp[5:13])))
            if status == STATUS_OK:
                answers[label] = rsp
            time.sleep(0.05)
    finally:
        if fd is not None:
            os.close(fd)

    cap = answers.get('system usage mode capability')
    cur = answers.get('current system usage mode')
    bat = answers.get('battery information')
    bnd = answers.get('battery boundary')
    if answers:
        out('\n## summary')
    if cap:
        out('mode capability level: %d -> %s' % (cap[5], MODE_SETS.get(cap[5], 'unknown level')))
    if cur:
        out('current mode byte: 0x%02X' % cur[5])
    ac = answers.get('AC status')
    if ac:
        # 0x04 on the charger and 0x00 on battery on the Aspire AG15-71P.
        out('AC status byte: 0x%02X' % ac[5])
    if bat:
        # Smart Battery registers as LE u16, offsets matched against
        # /sys/class/power_supply and the WMI battery readings on the Aspire
        # AG15-71P. Byte 32 (serial number) is masked above. The current is
        # positive while discharging (no sign convention seen yet for charging).
        u16 = lambda o: bat[o] | bat[o + 1] << 8
        cur_ma = u16(14) - 0x10000 if u16(14) & 0x8000 else u16(14)
        out('battery: %d mV, %d mA, %d mAh remaining, %d mAh full, %.1f °C' % (
            u16(12), cur_ma, u16(16), u16(20), u16(10) / 10 - 273.15))
        out('battery design: %d mAh, %d mV; cells: %s mV' % (
            u16(26), u16(28), ' '.join(str(u16(o)) for o in (36, 38, 40))))
    if bnd:
        # Enabled flag, upper and lower limit (only zeros seen so far).
        out('battery boundary: enabled=%d upper=%d lower=%d' % (bnd[7] == 1, bnd[8], bnd[9]))

    if opt.output:
        with open(opt.output, 'w') as f:
            f.write('\n'.join(lines) + '\n')
    return 0


if __name__ == '__main__':
    sys.exit(main())
