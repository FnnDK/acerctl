obj-m += acer-wmi-extras.o
KDIR ?= /lib/modules/$(shell uname -r)/build
PWD := $(CURDIR)

# Kernels built with clang (CachyOS's default one) need the LLVM toolchain:
# gcc rejects their flags. Checked when run directly, not when kbuild reads
# this file back; an LLVM= given on the command line wins.
ifeq ($(KERNELRELEASE),)
ifneq ($(shell grep -s '^CONFIG_CC_IS_CLANG=y' $(KDIR)/.config),)
LLVM ?= 1
export LLVM
endif
endif

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean
