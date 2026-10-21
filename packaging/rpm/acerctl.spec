# Built from the source archive of tools/release.sh (vendored Go modules and
# the frontend already built), so nothing is downloaded during the build.
# The kernel modules are in acerctl-kmod.spec (akmod-acerctl).
%global debug_package %{nil}

Name:           acerctl
Version:        0.1.9
Release:        1%{?dist}
Summary:        Daemon and CLI for the firmware features of Acer laptops

License:        GPL-3.0-or-later
URL:            https://github.com/FnnDK/acerctl
Source0:        %{url}/releases/download/v%{version}/acerctl-%{version}.tar.gz

ExclusiveArch:  x86_64
BuildRequires:  golang >= 1.26
BuildRequires:  gcc
BuildRequires:  pkgconfig(gtk+-3.0)
BuildRequires:  pkgconfig(webkit2gtk-4.1)
BuildRequires:  systemd-rpm-macros
BuildRequires:  desktop-file-utils
Requires:       dbus
Requires:       polkit
Recommends:     akmod-acerctl
%{?systemd_requires}

%description
acerd is a system daemon that exposes the firmware features of Acer laptops
(operating modes, fans, keyboard backlight, USB charging, battery charge limit
and calibration, RGB lighting where present) over D-Bus, next to the in-tree
acer_wmi driver; acerctl is its command-line client. Each feature appears only
when the firmware offers it.

%package        tray
Summary:        Tray icon and Sense key agent of acerctl
Requires:       %{name}%{?_isa} = %{version}-%{release}

%description    tray
acer-tray is the session agent of acerctl: tray icon with quick modes and
scenarios, and the Sense key's "open the app". It is started by the app
packages (nitro-, predator- or acer-control-center).

%package     -n acerctl-kmod-common
Summary:        Common files of the acerctl kernel modules
BuildArch:      noarch
Requires:       %{name} = %{version}-%{release}

%description -n acerctl-kmod-common
Loads acer-wmi-extras at boot (acer-wmi-battery loads through its WMI alias).
The modules themselves come from akmod-acerctl.

%package     -n nitro-control-center
Summary:        Nitro Control Center, the acerctl app for Acer Nitro laptops
Requires:       %{name}%{?_isa} = %{version}-%{release}
Requires:       %{name}-tray%{?_isa} = %{version}-%{release}
Conflicts:      predator-control-center
Conflicts:      acer-control-center

%description -n nitro-control-center
Desktop app of acerctl with the Nitro branding (red [N]). The same app ships
as predator-control-center and acer-control-center; install the one of your
laptop's product line.

%package     -n predator-control-center
Summary:        Predator Control Center, the acerctl app for Acer Predator laptops
Requires:       %{name}%{?_isa} = %{version}-%{release}
Requires:       %{name}-tray%{?_isa} = %{version}-%{release}
Conflicts:      nitro-control-center
Conflicts:      acer-control-center

%description -n predator-control-center
Desktop app of acerctl with the Predator branding (blue-grey [P]). The same app
ships as nitro-control-center and acer-control-center; install the one of your
laptop's product line.

%package     -n acer-control-center
Summary:        Acer Control Center, the acerctl app for the other Acer lines
Requires:       %{name}%{?_isa} = %{version}-%{release}
Requires:       %{name}-tray%{?_isa} = %{version}-%{release}
Conflicts:      nitro-control-center
Conflicts:      predator-control-center

%description -n acer-control-center
Desktop app of acerctl with the Acer branding (green [a]), for Aspire, Swift
and the other non-gaming lines. The same app ships as nitro-control-center and
predator-control-center; install the one of your laptop's product line.

%prep
%autosetup -n acerctl-%{version}

%build
export GOFLAGS="-mod=vendor -trimpath -buildmode=pie"
export CGO_CFLAGS="%{optflags} -Wno-deprecated-declarations"
export CGO_LDFLAGS="%{build_ldflags}"
LDFLAGS="-linkmode=external -B gobuildid"
mkdir -p _bin
# go-nvml leaves the nvml* symbols undefined and relies on lazy binding: they
# resolve only after it dlopens libnvidia-ml.so.1. With the distro's -z now,
# ld.so resolves them at startup and acerd exits 127 ("undefined symbol").
(cd daemon && CGO_LDFLAGS="$CGO_LDFLAGS -Wl,-z,lazy" go build -ldflags "$LDFLAGS -X main.Version=%{version}" -o ../_bin/acerd ./cmd/acerd)
(cd daemon && go build -ldflags "$LDFLAGS" -o ../_bin/acerctl ./cmd/acerctl)
(cd daemon && go build -ldflags "$LDFLAGS" -o ../_bin/acer-tray ./cmd/acer-tray)
# what "wails build" does, without the wails CLI: the frontend is already in gui/frontend/dist
(cd gui && go build -tags desktop,production,webkit2_41 -ldflags "$LDFLAGS" -o ../_bin/control-center .)

%install
install -Dpm 0755 _bin/acerd %{buildroot}%{_bindir}/acerd
install -Dpm 0755 _bin/acerctl %{buildroot}%{_bindir}/acerctl
install -Dpm 0755 _bin/acer-tray %{buildroot}%{_bindir}/acer-tray
install -Dpm 0644 packaging/acerd.service %{buildroot}%{_unitdir}/acerd.service
sed -i 's|/usr/local/bin/|%{_bindir}/|' %{buildroot}%{_unitdir}/acerd.service
install -Dpm 0644 packaging/io.github.fnndk.Acerd1.conf %{buildroot}%{_datadir}/dbus-1/system.d/io.github.fnndk.Acerd1.conf
install -Dpm 0644 packaging/io.github.fnndk.Acerd1.service %{buildroot}%{_datadir}/dbus-1/system-services/io.github.fnndk.Acerd1.service
install -Dpm 0644 packaging/80-acerctl.preset %{buildroot}%{_presetdir}/80-acerctl.preset
install -Dpm 0644 packaging/io.github.fnndk.acerd.policy %{buildroot}%{_datadir}/polkit-1/actions/io.github.fnndk.acerd.policy
install -Dpm 0644 packaging/61-acerctl-keyboard.hwdb %{buildroot}%{_udevhwdbdir}/61-acerctl-keyboard.hwdb
install -dm 0755 %{buildroot}%{_modulesloaddir}
echo acer_wmi_extras > %{buildroot}%{_modulesloaddir}/acerctl.conf

# one copy of the app per flavor: it picks its branding from its own file name
for f in Nitro:nitro:n Predator:predator:p Acer:acer:a; do
  name=${f%%%%:*}; rest=${f#*:}; bin=${rest%%:*}-control-center; logo=${rest#*:}
  id=io.github.fnndk.${name}ControlCenter
  install -Dpm 0755 _bin/control-center %{buildroot}%{_bindir}/$bin
  desktop-file-install --dir=%{buildroot}%{_datadir}/applications packaging/$id.desktop
  install -Dpm 0644 assets/logo/$logo-icon-cut.svg %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/$id.svg
  # the tray agent starts with the session of whichever app is installed
  sed -e "s|^Name=.*|Name=${name} Control Center (tray)|" -e "s|^Icon=.*|Icon=$id|" \
      -e 's|/usr/local/bin/|%{_bindir}/|' packaging/io.github.fnndk.AcerTray.desktop > tray.desktop
  install -Dpm 0644 tray.desktop %{buildroot}%{_sysconfdir}/xdg/autostart/$id.Tray.desktop
done

%check
# acerd must load without the NVIDIA driver (see -z lazy in %build)
./_bin/acerd -h 2>/dev/null
desktop-file-validate %{buildroot}%{_datadir}/applications/*.desktop

%post
%systemd_post acerd.service
%udev_hwdb_update

%preun
%systemd_preun acerd.service

%postun
%systemd_postun_with_restart acerd.service
%udev_hwdb_update

%files
%license LICENSE
%doc README.md docs/api-dbus.md docs/firmware-wmi.md
%{_bindir}/acerd
%{_bindir}/acerctl
%{_unitdir}/acerd.service
%{_datadir}/dbus-1/system.d/io.github.fnndk.Acerd1.conf
%{_datadir}/dbus-1/system-services/io.github.fnndk.Acerd1.service
%{_presetdir}/80-acerctl.preset
%{_datadir}/polkit-1/actions/io.github.fnndk.acerd.policy
%{_udevhwdbdir}/61-acerctl-keyboard.hwdb

%files tray
%license LICENSE
%{_bindir}/acer-tray

%files -n acerctl-kmod-common
%license drivers/LICENSE
%{_modulesloaddir}/acerctl.conf

%files -n nitro-control-center
%license LICENSE
%{_bindir}/nitro-control-center
%{_datadir}/applications/io.github.fnndk.NitroControlCenter.desktop
%{_datadir}/icons/hicolor/scalable/apps/io.github.fnndk.NitroControlCenter.svg
%config(noreplace) %{_sysconfdir}/xdg/autostart/io.github.fnndk.NitroControlCenter.Tray.desktop

%files -n predator-control-center
%license LICENSE
%{_bindir}/predator-control-center
%{_datadir}/applications/io.github.fnndk.PredatorControlCenter.desktop
%{_datadir}/icons/hicolor/scalable/apps/io.github.fnndk.PredatorControlCenter.svg
%config(noreplace) %{_sysconfdir}/xdg/autostart/io.github.fnndk.PredatorControlCenter.Tray.desktop

%files -n acer-control-center
%license LICENSE
%{_bindir}/acer-control-center
%{_datadir}/applications/io.github.fnndk.AcerControlCenter.desktop
%{_datadir}/icons/hicolor/scalable/apps/io.github.fnndk.AcerControlCenter.svg
%config(noreplace) %{_sysconfdir}/xdg/autostart/io.github.fnndk.AcerControlCenter.Tray.desktop

%changelog
* Wed Oct 21 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.9-1
- acerd retries the firmware's profile list for up to 2 s before taking the
  driver's, so a failed read right after boot no longer offers Turbo on
  laptops whose firmware has none

* Tue Oct 20 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.8-1
- 0.1.7 was tagged but never released (its build failed on the test
  transcripts); 0.1.8 carries its changes:
- acerd keeps the desktop power mode in sync when tuned-ppd starts in the same
  instant at boot: the unit is ordered after tuned-ppd and acerd connects to a
  power-profiles service that appears later
- acerctl status and the app's About show whether the power mode is in sync
- Setup is re-read when the app opens; the modules item tells a missing
  package, missing kernel headers and a failed build apart

* Mon Oct 19 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.6-1
- acerd keeps the laptop awake (logind inhibitor) while the battery calibrates
- App: the battery care card hides when the firmware offers neither the charge
  limit nor calibration

* Sun Oct 18 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.5-1
- acerctl setup: the app's setup steps in the CLI (acer_wmi option, Secure Boot
  key, kernel modules); acerctl status shows when something is left

* Sun Oct 18 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.4-1
- Released with acerctl-kmod 0.1.4; acerd --root DIR reads the kernel files
  under DIR (the known-model tests)

* Sat Oct 17 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.3-1
- Released with acerctl-kmod 0.1.3; no changes in these packages

* Sat Oct 17 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.2-1
- acerd starts on demand (D-Bus activation) and is enabled at install (preset)
- Setup screen: acer_wmi option and Secure Boot key enrollment from the app
- acerd starts again: built with lazy binding (it exited 127 under -z now)

* Wed Oct 14 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.1-1
- First RPM: daemon, CLI, tray, the three app flavors and kmod-common
