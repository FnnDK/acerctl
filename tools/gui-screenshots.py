#!/usr/bin/env python3
"""Screenshots of the GUI for the README (docs/screenshots/<flavor>/).

Starts acerd on the session bus, stages it, runs the GUI under `wails dev`
in the chosen flavor and captures every page its menu shows through headless
Chrome (DevTools Protocol), then stops everything it started:

  python3 -I tools/gui-screenshots.py nitro             # simulated ANV15-52
  python3 -I tools/gui-screenshots.py predator          # simulated PHN16-72 (Predator mock)
  python3 -I tools/gui-screenshots.py acer --dry-run    # this machine, read-only

Screenshots are always in English (a temporary XDG_CONFIG_HOME with
"lang": "en"; the user's settings are not touched), 1216x796 with the window's
transparent shadow margin. Simulated daemons are staged like a real machine in
use: Performance mode, fans at max and three scenarios with App Sync on. The
Monitor page collects --monitor-wait seconds of history and is shown on its
2 min window. `wails dev` also opens its own window on the desktop meanwhile.

Needs: daemon/acerd and daemon/acerctl built (sh packaging/install-daemon.sh
build), wails, google-chrome (or chromium) and python3-websocket.
"""
import argparse, base64, json, os, shutil, signal, subprocess, sys, tempfile, time, urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEV_URL = 'http://localhost:34115/'
SIZE = (1216, 796)  # window 1180x760 + 18 px shadow margin on each side
SIM = {'nitro': 'ANV15-52', 'predator': 'PHN16-72'}

# Staging of a simulated machine: (acerctl arguments...)
STAGE = [
    ['scenario', 'save', 'Gaming', '--mode-ac', 'balanced-performance', '--mode-battery', 'balanced',
     '--fan', 'cpu=max', '--fan', 'gpu=max', '--kbd-timeout', '0',
     '--app', 'eldenring.exe', '--app', 'cyberpunk2077.exe', '--app', 'cs2'],
    ['scenario', 'save', 'Rendering', '--mode-ac', 'balanced-performance', '--fan', 'cpu=80', '--fan', 'gpu=70',
     '--app', 'blender'],
    ['scenario', 'save', 'Quiet', '--mode-ac', 'quiet', '--mode-battery', 'low-power',
     '--fan', 'cpu=auto', '--fan', 'gpu=auto', '--kbd-timeout', '30'],
    ['appsync', 'on'],
    ['scenario', 'apply', 'Gaming'],
]


def die(msg):
    sys.exit('gui-screenshots: ' + msg)


def session_bus_has(name):
    r = subprocess.run(['busctl', '--user', 'call', 'org.freedesktop.DBus', '/org/freedesktop/DBus',
                        'org.freedesktop.DBus', 'NameHasOwner', 's', name], capture_output=True, text=True)
    return r.stdout.strip() == 'b true'


def page_html():
    try:
        return urllib.request.urlopen(DEV_URL, timeout=2).read().decode('utf-8', 'replace')
    except OSError:
        return ''


def wait_for(cond, secs, what):
    end = time.time() + secs
    while time.time() < end:
        if cond():
            return
        time.sleep(1)
    die('timed out waiting for ' + what)


def capture(out, monitor_wait):
    import websocket  # python3-websocket
    chrome_bin = shutil.which('google-chrome') or shutil.which('chromium') or die('google-chrome or chromium not found')
    port = 9333
    prof = tempfile.mkdtemp(prefix='gui-shots-chrome-')
    chrome = subprocess.Popen([chrome_bin, '--headless=new', f'--remote-debugging-port={port}', f'--user-data-dir={prof}',
                               f'--window-size={SIZE[0]},{SIZE[1]}', '--hide-scrollbars', '--no-first-run', '--disable-gpu',
                               'about:blank'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        targets = None
        for _ in range(50):
            try:
                targets = json.load(urllib.request.urlopen(f'http://127.0.0.1:{port}/json'))
                break
            except OSError:
                time.sleep(0.2)
        page = next(t for t in targets or [] if t['type'] == 'page')
        ws = websocket.create_connection(page['webSocketDebuggerUrl'], timeout=60, suppress_origin=True)
        seq = 0

        def cdp(method, **params):
            nonlocal seq
            seq += 1
            ws.send(json.dumps({'id': seq, 'method': method, 'params': params}))
            while True:
                m = json.loads(ws.recv())
                if m.get('id') == seq:
                    if 'error' in m:
                        raise RuntimeError(m['error'])
                    return m.get('result', {})

        def js(expr):
            return cdp('Runtime.evaluate', expression=expr, returnByValue=True).get('result', {}).get('value')

        cdp('Emulation.setDeviceMetricsOverride', width=SIZE[0], height=SIZE[1], deviceScaleFactor=1, mobile=False)
        cdp('Emulation.setDefaultBackgroundColorOverride', color={'r': 0, 'g': 0, 'b': 0, 'a': 0})
        cdp('Page.enable')
        cdp('Page.navigate', url=DEV_URL + '#monitor')
        time.sleep(6)  # the page reloads once to pick up the language from settings.json
        lang = js('document.documentElement.lang')
        if lang != 'en':
            die(f'the GUI came up in {lang!r}, not English')
        pages = js("[...document.querySelectorAll('[data-nav]')].filter(b => !b.classList.contains('hidden')).map(b => b.dataset.nav)")
        print('flavor', js('document.documentElement.dataset.flavor'), 'pages', ','.join(pages))
        js("document.querySelector('[data-w=\"120\"]')?.click()")
        print(f'collecting {monitor_wait} s of monitor history...', flush=True)
        time.sleep(monitor_wait)
        os.makedirs(out, exist_ok=True)
        for p in pages:
            js(f"location.hash = '#{p}'")
            time.sleep(3)
            js('document.activeElement && document.activeElement.blur()')
            shot = cdp('Page.captureScreenshot', format='png')
            path = os.path.join(out, p + '.png')
            with open(path, 'wb') as f:
                f.write(base64.b64decode(shot['data']))
            print('wrote', os.path.relpath(path, ROOT))
        ws.close()
    finally:
        chrome.terminate()
        chrome.wait()
        shutil.rmtree(prof, ignore_errors=True)


def main():
    ap = argparse.ArgumentParser(description=__doc__.split('\n')[0])
    ap.add_argument('flavor', choices=['acer', 'nitro', 'predator'])
    ap.add_argument('--dry-run', action='store_true', help="use this machine's hardware read-only instead of a simulated model")
    ap.add_argument('--out', help='output directory (default: docs/screenshots/<flavor>)')
    ap.add_argument('--monitor-wait', type=int, default=95, help='seconds of Monitor history before capturing (default 95)')
    opt = ap.parse_args()
    if not opt.dry_run and opt.flavor not in SIM:
        die(f'no simulated model for {opt.flavor}; run it on such a machine with --dry-run')
    out = opt.out or os.path.join(ROOT, 'docs', 'screenshots', opt.flavor)
    acerd, acerctl = os.path.join(ROOT, 'daemon', 'acerd'), os.path.join(ROOT, 'daemon', 'acerctl')
    wails = shutil.which('wails') or os.path.expanduser('~/go/bin/wails')
    for f in (acerd, acerctl, wails):
        if not os.access(f, os.X_OK):
            die(f'{f} not found (see the requirements in the header)')
    if session_bus_has('io.github.fnndk.Acerd1'):
        die('an acerd already runs on the session bus; stop it first')
    if page_html():
        die(f'something already serves {DEV_URL}; stop the other wails dev first')

    tmp = tempfile.mkdtemp(prefix='gui-shots-')
    cfg = os.path.join(tmp, 'config')
    os.makedirs(os.path.join(cfg, 'acerctl'))
    with open(os.path.join(cfg, 'acerctl', 'settings.json'), 'w') as f:
        json.dump({'lang': 'en'}, f)
    env = dict(os.environ, XDG_CONFIG_HOME=cfg, ACERCTL_SESSION='1', ACERCTL_FLAVOR=opt.flavor)
    env['PATH'] = env.get('PATH', '') + os.pathsep + os.path.dirname(wails)

    args = [acerd, '--session', '--no-polkit', '--state', os.path.join(tmp, 'state.json')]
    args += ['--dry-run'] if opt.dry_run else ['--simulate', SIM[opt.flavor]]
    log = open(os.path.join(tmp, 'acerd.log'), 'w')
    daemon = subprocess.Popen(args, stdout=log, stderr=log)
    dev = None
    try:
        wait_for(lambda: session_bus_has('io.github.fnndk.Acerd1'), 20, 'acerd')
        if not opt.dry_run:
            for a in STAGE:
                subprocess.run([acerctl, '--session'] + a, check=True, stdout=subprocess.DEVNULL)
        subprocess.run([acerctl, '--session', 'status'], check=True)
        print('starting wails dev (it also opens a window)...', flush=True)
        dev = subprocess.Popen([wails, 'dev', '-tags', 'webkit2_41'], cwd=os.path.join(ROOT, 'gui'), env=env,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
        wait_for(lambda: f'data-flavor="{opt.flavor}"' in page_html(), 300, 'wails dev')
        capture(out, opt.monitor_wait)
    finally:
        if dev:
            os.killpg(dev.pid, signal.SIGTERM)
            dev.wait()
        daemon.terminate()
        daemon.wait()
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == '__main__':
    main()
