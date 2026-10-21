// SPDX-License-Identifier: GPL-2.0
/*
 * acer-wmi-extras - complements the in-tree acer_wmi driver on Acer laptops
 * with "Sense" (Nitro/Predator) firmware, exposing features acer_wmi does not:
 *
 *   - keyboard backlight auto-off timeout            (APGeAction WMI, fn 1/2)
 *   - power-off USB charging threshold               (APGeAction WMI, fn 4)
 *   - thermal profiles the firmware really supports  (gaming WMI, misc 0x0A)
 *   - battery voltage / current / temperature hwmon  (battery WMI, SBS commands)
 *
 * It uses only the legacy GUID-based WMI API, binds to no WMI device and
 * coexists with acer_wmi and acer-wmi-battery. Every feature is probed at
 * load time and only exposed when the firmware answers with status 0, so
 * capability comes from the firmware, not from a model table. The keyboard
 * timeout call also answers 0 on laptops without a keyboard backlight, so it
 * additionally needs the backlight function in the firmware's DMI table.
 *
 * Protocol notes (reverse engineered on a Nitro ANV15-52, see docs/firmware-wmi.md):
 *   All answers are little-endian buffers whose byte 0 is a status
 *   (0 = ok, 1 = unsupported, 2 = invalid parameter).
 */
#include <linux/module.h>
#include <linux/acpi.h>
#include <linux/dmi.h>
#include <linux/wmi.h>
#include <linux/platform_device.h>
#include <linux/hwmon.h>
#include <linux/mutex.h>
#include <linux/slab.h>
#include <linux/sysfs.h>
#include <linux/version.h>
/* moved from asm/ to linux/ in 6.12 */
#if LINUX_VERSION_CODE < KERNEL_VERSION(6,12,0)
#include <asm/unaligned.h>
#else
#include <linux/unaligned.h>
#endif

#define DRV_NAME	"acer-wmi-extras"

#define GAMING_GUID	"7A4DDFE7-5B5D-40B4-8595-4408E0CC7F56"
#define APGE_GUID	"61EF69EA-865C-4BC3-A502-A0DEBA0CB531"
#define BATTERY_GUID	"79772EC5-04B1-4BFD-843C-61E7F77B6CC9"

/* AcerGamingFunction */
#define GAMING_GET_MISC_SETTING		23
#define MISC_SUPPORTED_PROFILES		0x0A

/* APGeAction */
#define APGE_SET_FUNCTION		1
#define APGE_GET_FUNCTION		2
#define APGE_KBD_GET			0x088401ULL	/* fn 1, selector 84 08 */
#define APGE_KBD_SET			0x088402ULL	/* fn 2, selector 84 08 */
#define APGE_KBD_ON_BIT			0x08		/* byte 2 */
#define APGE_KBD_TIMEOUT_SHIFT		40		/* byte 5: seconds */
#define APGE_USB_FN			0x04ULL
#define APGE_USB_ENABLED		0x0f		/* byte 1 */
#define APGE_USB_DISABLED		0x1f
#define APGE_USB_THRESHOLD_SHIFT	16		/* byte 2: percent */

/* BatteryControl */
#define BATTERY_GET_INFO		19
#define BATTERY_NO			1
#define SBS_TEMPERATURE			0x08		/* 0.1 K */
#define SBS_VOLTAGE			0x09		/* mV */
#define SBS_CURRENT			0x0A		/* mA, signed */

/*
 * DMI OEM type 0xAA, the hotkey function table acer_wmi also reads: after the
 * header, five u16 bitmaps (communication, application, media, display,
 * others) of the functions present, then entries { u8 code, u8 2, u16 bit }
 * giving each function's bit. Code 0x84 is the keyboard backlight, the same
 * code as its hotkey event and as the APGe selector.
 */
#define DMI_ACER_HOTKEYS		0xAA
#define DMI_AA_OTHERS			12
#define DMI_AA_ENTRIES			14
#define DMI_AA_KBD_BACKLIGHT		0x84

/* Acer thermal profile bits in the supported-profiles mask */
#define ACER_PROFILE_QUIET		BIT(0)
#define ACER_PROFILE_BALANCED		BIT(1)
#define ACER_PROFILE_PERFORMANCE	BIT(4)
#define ACER_PROFILE_TURBO		BIT(5)
#define ACER_PROFILE_ECO		BIT(6)

struct extras_data {
	bool has_kbd_timeout;
	bool has_usb_charging;
	bool has_gaming;
	bool has_battery;
	int profiles_err;	/* of the read at probe, for the log only */
	u8 profiles_mask;
};

static DEFINE_MUTEX(extras_lock);

/* --- generic WMI call: input bytes in, first 4/8 output bytes as u64 out --- */
static int extras_wmi_call(const char *guid, u32 method, const void *in,
			   size_t in_len, u64 *out)
{
	struct acpi_buffer input = { in_len, (void *)in };
	struct acpi_buffer output = { ACPI_ALLOCATE_BUFFER, NULL };
	union acpi_object *obj;
	acpi_status status;
	int ret = 0;

	mutex_lock(&extras_lock);
	status = wmi_evaluate_method(guid, 0, method, &input, &output);
	mutex_unlock(&extras_lock);
	if (ACPI_FAILURE(status))
		return -EIO;

	obj = output.pointer;
	if (!obj)
		return -ENODATA;
	switch (obj->type) {
	case ACPI_TYPE_BUFFER:
		if (obj->buffer.length >= 8)
			*out = get_unaligned_le64(obj->buffer.pointer);
		else if (obj->buffer.length >= 4)
			*out = get_unaligned_le32(obj->buffer.pointer);
		else
			ret = -EPROTO;
		break;
	case ACPI_TYPE_INTEGER:
		*out = obj->integer.value;
		break;
	default:
		ret = -EPROTO;
	}
	kfree(obj);
	return ret;
}

static int extras_call_u32(const char *guid, u32 method, u32 in, u64 *out)
{
	u8 buf[4];

	put_unaligned_le32(in, buf);
	return extras_wmi_call(guid, method, buf, sizeof(buf), out);
}

static int extras_call_u64(const char *guid, u32 method, u64 in, u64 *out)
{
	u8 buf[8];

	put_unaligned_le64(in, buf);
	return extras_wmi_call(guid, method, buf, sizeof(buf), out);
}

/* Returns 0 on firmware status 0, -EOPNOTSUPP on 1, -EINVAL on 2, -EIO else */
static int extras_status(u64 out)
{
	switch (out & 0xff) {
	case 0:
		return 0;
	case 1:
		return -EOPNOTSUPP;
	case 2:
		return -EINVAL;
	default:
		return -EIO;
	}
}

/* ------------------------------------------------------------------ */
/* Keyboard backlight timeout                                           */

static int kbd_get(u8 *on, u8 *timeout)
{
	u64 out;
	int ret;

	ret = extras_call_u32(APGE_GUID, APGE_GET_FUNCTION, APGE_KBD_GET, &out);
	if (ret)
		return ret;
	ret = extras_status(out);
	if (ret)
		return ret;
	if (on)
		*on = !!((out >> 16) & APGE_KBD_ON_BIT);
	if (timeout)
		*timeout = (out >> APGE_KBD_TIMEOUT_SHIFT) & 0xff;
	return 0;
}

static int kbd_set_timeout(u8 seconds)
{
	u64 in = APGE_KBD_SET | ((u64)seconds << APGE_KBD_TIMEOUT_SHIFT);
	u64 out;
	int ret;

	ret = extras_call_u64(APGE_GUID, APGE_SET_FUNCTION, in, &out);
	if (ret)
		return ret;
	return extras_status(out);
}

struct kbd_dmi {
	bool listed;
	bool present;
};

static void kbd_dmi_decode(const struct dmi_header *dh, void *priv)
{
	const u8 *p = (const u8 *)dh;
	struct kbd_dmi *k = priv;
	u16 others;
	int i;

	if (dh->type != DMI_ACER_HOTKEYS || dh->length < DMI_AA_ENTRIES)
		return;
	others = get_unaligned_le16(p + DMI_AA_OTHERS);
	for (i = DMI_AA_ENTRIES; i + 4 <= dh->length; i += 4) {
		if (p[i] == DMI_AA_KBD_BACKLIGHT && p[i + 1] == 2) {
			k->listed = true;
			k->present = others & get_unaligned_le16(p + i + 2);
			return;
		}
	}
}

/* false only when the DMI table lists the backlight function as absent */
static bool kbd_backlight_in_dmi(void)
{
	struct kbd_dmi k = {};

	if (dmi_walk(kbd_dmi_decode, &k) || !k.listed)
		return true;
	return k.present;
}

static ssize_t kbd_backlight_timeout_show(struct device *dev,
					  struct device_attribute *attr, char *buf)
{
	u8 timeout;
	int ret = kbd_get(NULL, &timeout);

	if (ret)
		return ret;
	return sysfs_emit(buf, "%u\n", timeout);
}

static ssize_t kbd_backlight_timeout_store(struct device *dev,
					   struct device_attribute *attr,
					   const char *buf, size_t count)
{
	unsigned int seconds;
	int ret;

	ret = kstrtouint(buf, 0, &seconds);
	if (ret)
		return ret;
	/* Only the values NitroSense uses are known to be accepted. */
	if (seconds != 0 && seconds != 30)
		return -EINVAL;
	ret = kbd_set_timeout(seconds);
	return ret ? ret : count;
}
static DEVICE_ATTR_RW(kbd_backlight_timeout);

static ssize_t kbd_backlight_state_show(struct device *dev,
					struct device_attribute *attr, char *buf)
{
	u8 on;
	int ret = kbd_get(&on, NULL);

	if (ret)
		return ret;
	return sysfs_emit(buf, "%u\n", on);
}
static DEVICE_ATTR_RO(kbd_backlight_state);

/* ------------------------------------------------------------------ */
/* Power-off USB charging                                               */

static int usb_get(u8 *percent)
{
	u64 out;
	int ret;

	ret = extras_call_u32(APGE_GUID, APGE_GET_FUNCTION, APGE_USB_FN, &out);
	if (ret)
		return ret;
	ret = extras_status(out);
	if (ret)
		return ret;
	if (((out >> 8) & 0xff) == APGE_USB_DISABLED)
		*percent = 0;
	else
		*percent = (out >> APGE_USB_THRESHOLD_SHIFT) & 0xff;
	return 0;
}

static int usb_set(u8 percent)
{
	u64 in = APGE_USB_FN;
	u64 out;
	int ret;

	if (percent == 0)
		in |= ((u64)APGE_USB_DISABLED << 8) | (10ULL << APGE_USB_THRESHOLD_SHIFT);
	else
		in |= ((u64)APGE_USB_ENABLED << 8) | ((u64)percent << APGE_USB_THRESHOLD_SHIFT);

	ret = extras_call_u64(APGE_GUID, APGE_SET_FUNCTION, in, &out);
	if (ret)
		return ret;
	return extras_status(out);
}

static ssize_t usb_charging_show(struct device *dev,
				 struct device_attribute *attr, char *buf)
{
	u8 percent;
	int ret = usb_get(&percent);

	if (ret)
		return ret;
	return sysfs_emit(buf, "%u\n", percent);
}

static ssize_t usb_charging_store(struct device *dev,
				  struct device_attribute *attr,
				  const char *buf, size_t count)
{
	unsigned int percent;
	int ret;

	ret = kstrtouint(buf, 0, &percent);
	if (ret)
		return ret;
	if (percent != 0 && percent != 10 && percent != 20 && percent != 30)
		return -EINVAL;
	ret = usb_set(percent);
	return ret ? ret : count;
}
static DEVICE_ATTR_RW(usb_charging);

/* ------------------------------------------------------------------ */
/* Supported thermal profiles                                           */

static int profiles_get(u8 *mask)
{
	u64 out;
	int ret;

	ret = extras_call_u32(GAMING_GUID, GAMING_GET_MISC_SETTING,
			      MISC_SUPPORTED_PROFILES, &out);
	if (ret)
		return ret;
	ret = extras_status(out);
	if (ret)
		return ret;
	*mask = (out >> 8) & 0xff;
	return 0;
}

static ssize_t supported_profiles_mask_show(struct device *dev,
					    struct device_attribute *attr, char *buf)
{
	u8 mask;
	int ret = profiles_get(&mask);

	if (ret)
		return ret;
	return sysfs_emit(buf, "0x%02x\n", mask);
}
static DEVICE_ATTR_RO(supported_profiles_mask);

/* Names follow /sys/class/platform-profile as mapped by acer_wmi predator_v4 */
static ssize_t supported_profiles_show(struct device *dev,
				       struct device_attribute *attr, char *buf)
{
	static const struct { u8 bit; const char *name; } map[] = {
		{ ACER_PROFILE_ECO,		"low-power" },
		{ ACER_PROFILE_QUIET,		"quiet" },
		{ ACER_PROFILE_BALANCED,	"balanced" },
		{ ACER_PROFILE_PERFORMANCE,	"balanced-performance" },
		{ ACER_PROFILE_TURBO,		"performance" },
	};
	u8 mask;
	int ret, i, n = 0;

	ret = profiles_get(&mask);
	if (ret)
		return ret;
	for (i = 0; i < ARRAY_SIZE(map); i++)
		if (mask & map[i].bit)
			n += sysfs_emit_at(buf, n, "%s%s", n ? " " : "", map[i].name);
	n += sysfs_emit_at(buf, n, "\n");
	return n;
}
static DEVICE_ATTR_RO(supported_profiles);

/* ------------------------------------------------------------------ */
/* Battery (Smart Battery commands through the battery WMI interface)   */

static int battery_info(u32 sbs_cmd, u32 *value)
{
	u8 buf[8];
	u64 out;
	int ret;

	put_unaligned_le32(sbs_cmd, buf);
	put_unaligned_le32(BATTERY_NO, buf + 4);
	ret = extras_wmi_call(BATTERY_GUID, BATTERY_GET_INFO, buf, sizeof(buf), &out);
	if (ret)
		return ret;
	*value = out & 0xffffffff;
	return 0;
}

static umode_t extras_hwmon_is_visible(const void *data,
				       enum hwmon_sensor_types type,
				       u32 attr, int channel)
{
	return 0444;
}

static int extras_hwmon_read(struct device *dev, enum hwmon_sensor_types type,
			     u32 attr, int channel, long *val)
{
	u32 v;
	int ret;

	switch (type) {
	case hwmon_temp:
		ret = battery_info(SBS_TEMPERATURE, &v);
		if (ret)
			return ret;
		if (v > U16_MAX || v < 2000)
			return -ENODATA;
		*val = ((long)v - 2731) * 100;	/* 0.1 K -> millidegree C */
		return 0;
	case hwmon_in:
		ret = battery_info(SBS_VOLTAGE, &v);
		if (ret)
			return ret;
		if (v > U16_MAX)
			return -ENODATA;
		*val = v;			/* mV */
		return 0;
	case hwmon_curr:
		ret = battery_info(SBS_CURRENT, &v);
		if (ret)
			return ret;
		if (v > U16_MAX)
			return -ENODATA;
		*val = (s16)v;			/* mA, negative = discharging */
		return 0;
	default:
		return -EOPNOTSUPP;
	}
}

static int extras_hwmon_read_string(struct device *dev,
				    enum hwmon_sensor_types type, u32 attr,
				    int channel, const char **str)
{
	*str = "battery";
	return 0;
}

/* hwmon_chip_info.info points to const pointers from 6.4 on */
#if LINUX_VERSION_CODE < KERNEL_VERSION(6,4,0)
static const struct hwmon_channel_info *extras_hwmon_info[] = {
#else
static const struct hwmon_channel_info *const extras_hwmon_info[] = {
#endif
	HWMON_CHANNEL_INFO(temp, HWMON_T_INPUT | HWMON_T_LABEL),
	HWMON_CHANNEL_INFO(in, HWMON_I_INPUT | HWMON_I_LABEL),
	HWMON_CHANNEL_INFO(curr, HWMON_C_INPUT | HWMON_C_LABEL),
	NULL
};

static const struct hwmon_ops extras_hwmon_ops = {
	.is_visible = extras_hwmon_is_visible,
	.read = extras_hwmon_read,
	.read_string = extras_hwmon_read_string,
};

static const struct hwmon_chip_info extras_hwmon_chip = {
	.ops = &extras_hwmon_ops,
	.info = extras_hwmon_info,
};

/* ------------------------------------------------------------------ */
/* Platform device                                                      */

static umode_t extras_attr_is_visible(struct kobject *kobj,
				      struct attribute *attr, int n)
{
	struct device *dev = kobj_to_dev(kobj);
	struct extras_data *d = dev_get_drvdata(dev);

	if (attr == &dev_attr_kbd_backlight_timeout.attr ||
	    attr == &dev_attr_kbd_backlight_state.attr)
		return d->has_kbd_timeout ? attr->mode : 0;
	if (attr == &dev_attr_usb_charging.attr)
		return d->has_usb_charging ? attr->mode : 0;
	/*
	 * Shown whenever the gaming interface exists, not only when the read at
	 * probe worked: right after boot the firmware can fail that call (seen
	 * on the ANV15-52 with the other Acer modules probing in the same
	 * second), and every read asks the firmware again anyway. Where the
	 * firmware has no profile mask, reads fail with -EOPNOTSUPP.
	 */
	if (attr == &dev_attr_supported_profiles.attr ||
	    attr == &dev_attr_supported_profiles_mask.attr)
		return d->has_gaming ? attr->mode : 0;
	return attr->mode;
}

static struct attribute *extras_attrs[] = {
	&dev_attr_kbd_backlight_timeout.attr,
	&dev_attr_kbd_backlight_state.attr,
	&dev_attr_usb_charging.attr,
	&dev_attr_supported_profiles.attr,
	&dev_attr_supported_profiles_mask.attr,
	NULL
};

static const struct attribute_group extras_group = {
	.attrs = extras_attrs,
	.is_visible = extras_attr_is_visible,
};
__ATTRIBUTE_GROUPS(extras);

static int extras_probe(struct platform_device *pdev)
{
	struct extras_data *d;
	struct device *hwmon;
	u32 v;
	u8 tmp;

	d = devm_kzalloc(&pdev->dev, sizeof(*d), GFP_KERNEL);
	if (!d)
		return -ENOMEM;

	if (wmi_has_guid(APGE_GUID)) {
		d->has_kbd_timeout = kbd_get(NULL, &tmp) == 0 && kbd_backlight_in_dmi();
		d->has_usb_charging = usb_get(&tmp) == 0;
	}
	d->has_gaming = wmi_has_guid(GAMING_GUID);
	d->profiles_err = d->has_gaming ? profiles_get(&d->profiles_mask) : -ENODEV;
	if (wmi_has_guid(BATTERY_GUID) && battery_info(SBS_TEMPERATURE, &v) == 0 &&
	    v >= 2000 && v <= U16_MAX)
		d->has_battery = true;

	platform_set_drvdata(pdev, d);

	if (d->has_battery) {
		hwmon = devm_hwmon_device_register_with_info(&pdev->dev, "acer_battery",
							     d, &extras_hwmon_chip, NULL);
		if (IS_ERR(hwmon))
			dev_warn(&pdev->dev, "battery hwmon registration failed: %ld\n",
				 PTR_ERR(hwmon));
	}

	if (d->profiles_err)
		dev_info(&pdev->dev,
			 "kbd timeout: %s, usb charging: %s, profiles: not read (%d)%s, battery: %s\n",
			 d->has_kbd_timeout ? "yes" : "no", d->has_usb_charging ? "yes" : "no",
			 d->profiles_err, d->has_gaming ? ", asked again on each read" : "",
			 d->has_battery ? "yes" : "no");
	else
		dev_info(&pdev->dev,
			 "kbd timeout: %s, usb charging: %s, profiles: mask 0x%02x, battery: %s\n",
			 d->has_kbd_timeout ? "yes" : "no", d->has_usb_charging ? "yes" : "no",
			 d->profiles_mask, d->has_battery ? "yes" : "no");
	return 0;
}

static struct platform_driver extras_driver = {
	.driver = {
		.name = DRV_NAME,
		.dev_groups = extras_groups,
	},
	.probe = extras_probe,
};

static struct platform_device *extras_pdev;

static int __init extras_init(void)
{
	int ret;

	if (!wmi_has_guid(APGE_GUID) && !wmi_has_guid(GAMING_GUID) &&
	    !wmi_has_guid(BATTERY_GUID)) {
		pr_info(DRV_NAME ": no Acer WMI interfaces found\n");
		return -ENODEV;
	}
	ret = platform_driver_register(&extras_driver);
	if (ret)
		return ret;
	extras_pdev = platform_device_register_simple(DRV_NAME, PLATFORM_DEVID_NONE,
						      NULL, 0);
	if (IS_ERR(extras_pdev)) {
		platform_driver_unregister(&extras_driver);
		return PTR_ERR(extras_pdev);
	}
	return 0;
}

static void __exit extras_exit(void)
{
	platform_device_unregister(extras_pdev);
	platform_driver_unregister(&extras_driver);
}

module_init(extras_init);
module_exit(extras_exit);

MODULE_AUTHOR("Fernando Klumpp");
MODULE_DESCRIPTION("Extra Acer gaming-laptop features complementing acer_wmi");
MODULE_LICENSE("GPL");
