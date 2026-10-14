# akmod package of the acerctl kernel modules (acer-wmi-extras, acer-wmi-battery),
# after the RPM Fusion kmod template. akmods builds kmod-acerctl for each new
# kernel on the machine and signs it with the local akmods key when one is set
# up (Secure Boot). Pass --define "kernels $(uname -r)" to build a kmod directly.
%if 0%{?fedora}
%global buildforkernels akmod
%endif
%global debug_package %{nil}

Name:           acerctl-kmod
Version:        0.1.1
Release:        1%{?dist}
Summary:        Kernel modules of acerctl (acer-wmi-extras, acer-wmi-battery)

# acer-wmi-extras: GPL-2.0-only; acer-wmi-battery: GPL-2.0-or-later
License:        GPL-2.0-only AND GPL-2.0-or-later
URL:            https://github.com/FnnDK/acerctl
Source0:        %{url}/releases/download/v%{version}/acerctl-%{version}.tar.gz

ExclusiveArch:  x86_64
BuildRequires:  kmodtool

%{expand:%(kmodtool --target %{_target_cpu} --kmodname %{name} %{?buildforkernels:--%{buildforkernels}} %{?kernels:--for-kernels "%{?kernels}"} 2>/dev/null) }

%description
Companion modules of acerctl, next to the in-tree acer_wmi driver:
acer-wmi-extras (keyboard backlight timeout, USB charging, supported thermal
profiles, battery sensors) and acer-wmi-battery (80 % charge limit through the
power_supply charge_control_end_threshold, calibration).

%prep
%{?kmodtool_check}
kmodtool --target %{_target_cpu} --kmodname %{name} %{?buildforkernels:--%{buildforkernels}} %{?kernels:--for-kernels "%{?kernels}"} 2>/dev/null
%setup -q -c
for kernel_version in %{?kernel_versions}; do
  cp -a acerctl-%{version}/drivers _kmod_build_${kernel_version%%___*}
done

%build
for kernel_version in %{?kernel_versions}; do
  for m in acer-wmi-extras acer-wmi-battery; do
    %make_build -C "${kernel_version##*___}" M="${PWD}/_kmod_build_${kernel_version%%___*}/$m" modules
  done
done

%install
for kernel_version in %{?kernel_versions}; do
  for m in acer-wmi-extras acer-wmi-battery; do
    install -D -m 0755 _kmod_build_${kernel_version%%___*}/$m/$m.ko \
      %{buildroot}%{kmodinstdir_prefix}/${kernel_version%%___*}/%{kmodinstdir_postfix}/$m.ko
  done
done
%{?akmod_install}

%changelog
* Wed Oct 14 2026 FnnDK <fernando.resende.k@gmail.com> - 0.1.1-1
- First akmod package: acer-wmi-extras and acer-wmi-battery
