#!/usr/bin/env python3
"""Read-only queries to the EC's vendor HID channel (report 0xA0 of 1025:174B).

Newer non-gaming Acer laptops (e.g. Aspire AG15-71P) expose operating modes and
battery settings through the EC's HID-over-I2C device 1025:174B instead of WMI.
Commands go out as a 65-byte feature report (report ID 0xA0) and the answer is
read back with a get-feature on the same report ID:

  request:  A0 | 00 A0 | id(lo) id(hi) | sub | args...
  response: A0 | status(lo) status(hi) (0xE000 = OK) | id echo (2 bytes) | data...

Only query sub-functions are sent: everything that sets a value (sub 0x01) is
refused, except for id 0x0008, where the sub byte selects what to read. The
battery serial number in the battery information answer is masked.
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
    # Not confirmed on hardware yet: expected layouts are in the summary code.
    ('OC profile',                   0x0002, 0x02, [0]),
    ('keyboard backlight',           0x000A, 0x02, [2]),   # device 2
    ('panel DFR mode',               0x000A, 0x02, [3]),   # device 3
    ('system health 1',              0x0008, 0x01, [0]),
    ('system health 2',              0x0008, 0x02, [0]),
    ('BSOC table row 1',             0x000B, 0x02, [1, 1]),
]

# Commands whose sub byte 0x01 is a read selector rather than a set.
SUB1_READS = {0x0008}
# Commands sent with the unused bytes set to 0xFF instead of 0x00.
PAD_FF = {0x000B}

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
    if sub == 0x01 and cmd_id not in SUB1_READS:
        raise ValueError('set sub-functions are not allowed')
    buf = bytearray([0xFF if cmd_id in PAD_FF else 0x00] * REPORT_LEN)
    body = [REPORT_ID, 0x00, 0xA0, cmd_id & 0xFF, cmd_id >> 8, sub] + args
    buf[:len(body)] = bytes(body)
    return buf


def hexs(b):
    return ' '.join('%02x' % x for x in b)


def le16(b, o):
    return b[o] | b[o + 1] << 8


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
                time.sleep(0.02)   # give the EC time to prepare the answer
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

    cur = answers.get('current system usage mode')
    bat = answers.get('battery information')
    bnd = answers.get('battery boundary')
    if answers:
        out('\n## summary')
    new_layout = False
    ver = answers.get('HID spec version')
    if ver:
        # The version is u16 5-6 when bytes 7-8 are 0xFFFF, otherwise u16 7-8
        # (with byte 5 = 0, the sub echo). Shown as low byte . high byte.
        # Expected: from 0.6 on, or with a non-zero low byte, the id 0x0000
        # answers echo the sub in byte 5 and carry the value in bytes 7-8
        # instead of 5-6 (the Aspire AG15-71P, old layout, has 00 04 in 5-6).
        version = (le16(ver, 5) if le16(ver, 7) == 0xFFFF else
                   le16(ver, 7) if ver[5] == 0 else None)
        if version is None:
            out('data version: unrecognized answer')
        else:
            new_layout = (version & 0xFF) != 0 or version >> 8 > 5
            out('data version: %d.%d (0x%04X) -> %s layout for id 0x0000 answers' % (
                version & 0xFF, version >> 8, version, 'new' if new_layout else 'old'))
    sub_of = {q[0]: q[2] for q in QUERIES}

    def system_value(label):
        r = answers.get(label)
        if r is None:
            return None
        if new_layout:
            return le16(r, 7) if r[5] == sub_of[label] else None
        return le16(r, 5)

    level = system_value('system usage mode capability')
    if level is not None:
        out('mode capability level: %d -> %s' % (level, MODE_SETS.get(level, 'unknown level')))
    if cur:
        out('current mode byte: 0x%02X' % cur[5])
    for label in ('battery boost status', 'AC status', 'OC profile capability',
                  'system usage mode limit', 'PD adaptor capability'):
        # AC status: 0x04 on the charger and 0x00 on battery on the Aspire AG15-71P.
        v = system_value(label)
        if v is not None:
            out('%s: 0x%04X' % (label, v))
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

    # Expected layouts below are not confirmed on hardware yet.
    ocp = answers.get('OC profile')
    if ocp:
        # Twelve fields of unknown meaning: byte 6, u16 7..15, bytes 17..19, u16 20..24.
        fields = ([ocp[6]] + [le16(ocp, o) for o in (7, 9, 11, 13, 15)] +
                  list(ocp[17:20]) + [le16(ocp, o) for o in (20, 22, 24)])
        out('OC profile fields: %s' % ' '.join(str(f) for f in fields))
    kb = answers.get('keyboard backlight')
    if kb and kb[6] == 2:
        # Device 2: u16 7-8 a flag (1 when set), 9-10 brightness, 11-12 auto-off in seconds.
        out('keyboard backlight: flag=%d brightness=%d auto-off=%d s' % (
            le16(kb, 7), le16(kb, 9), le16(kb, 11)))
    dfr = answers.get('panel DFR mode')
    if dfr and dfr[6] == 3:
        # Device 3: u16 7-8 the mode (1 or 2, meaning unknown).
        out('panel DFR mode: %d' % le16(dfr, 7))
    for label, off in (('system health 1', 6), ('system health 2', 8)):
        h = answers.get(label)
        if h:
            out('%s: 0x%04X' % (label, le16(h, off)))
    bsoc = answers.get('BSOC table row 1')
    if bsoc:
        # Byte 6 row count and byte 7 row echo (order not certain), then charge
        # in %, current in units of 100 mA and power in units of 1 W.
        out('BSOC table row 1: bytes 6-7 %s, %d %%, %d mA, %d mW' % (
            hexs(bsoc[6:8]), bsoc[8], bsoc[9] * 100, bsoc[10] * 1000))

    if opt.output:
        with open(opt.output, 'w') as f:
            f.write('\n'.join(lines) + '\n')
    return 0


if __name__ == '__main__':
    sys.exit(main())
