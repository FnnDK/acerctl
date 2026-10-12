// SPDX-License-Identifier: GPL-2.0
/*
 * acer-wmi-lab - research helper: evaluate arbitrary ACPI-WMI methods
 * from user space through a sysfs attribute.
 *
 * Usage (root):
 *   echo "<GUID> <method_id> [hex input bytes]" > /sys/devices/platform/acer-wmi-lab/call
 *   cat /sys/devices/platform/acer-wmi-lab/call
 *
 * The input bytes are passed verbatim as the ACPI buffer argument (Arg2 of
 * the WMxx method). With no bytes a 4-byte zero buffer is used. The result
 * of the last call (status, output object type and bytes) is kept until the
 * next call. Calls are serialized with a mutex. This module coexists with
 * acer_wmi: it only uses the legacy GUID-based API and binds to nothing.
 *
 * sysfs is used instead of debugfs because kernel lockdown (Secure Boot)
 * refuses to open writable debugfs files.
 *
 * NOT for production. Set methods can reconfigure the EC/firmware.
 */
#include <linux/module.h>
#include <linux/acpi.h>
#include <linux/wmi.h>
#include <linux/platform_device.h>
#include <linux/device.h>
#include <linux/mutex.h>
#include <linux/slab.h>
#include <linux/string.h>
#include <linux/hex.h>
#include <linux/unaligned.h>

#define LAB_MAX_IN	512
#define LAB_MAX_RESULT	PAGE_SIZE

static DEFINE_MUTEX(lab_lock);
static struct platform_device *lab_pdev;
static char *lab_result;
static size_t lab_result_len;

static __printf(1, 2) void lab_printf(const char *fmt, ...)
{
	va_list args;

	if (lab_result_len >= LAB_MAX_RESULT - 1)
		return;
	va_start(args, fmt);
	lab_result_len += vscnprintf(lab_result + lab_result_len,
				     LAB_MAX_RESULT - lab_result_len, fmt, args);
	va_end(args);
}

static void lab_hex(const u8 *buf, size_t len)
{
	size_t i;

	for (i = 0; i < len; i++) {
		if (lab_result_len >= LAB_MAX_RESULT - 8) {
			lab_printf("...");
			break;
		}
		lab_printf("%02x%s", buf[i], (i + 1 < len) ? " " : "");
	}
}

/* Parse "0a 1b2c:3d" style hex into out; returns byte count or -errno */
static int lab_parse_hex(const char *s, u8 *out, size_t max)
{
	size_t n = 0;
	int hi = -1;

	for (; *s; s++) {
		int v;

		if (*s == ' ' || *s == ':' || *s == ',' || *s == '\n' ||
		    *s == '\t' || *s == '\r')
			continue;
		v = hex_to_bin(*s);
		if (v < 0)
			return -EINVAL;
		if (hi < 0) {
			hi = v;
			continue;
		}
		if (n >= max)
			return -E2BIG;
		out[n++] = (hi << 4) | v;
		hi = -1;
	}
	if (hi >= 0)
		return -EINVAL;	/* odd number of digits */
	return n;
}

static void lab_dump_object(union acpi_object *obj, int depth)
{
	u32 i;

	if (!obj) {
		lab_printf("out: (null)\n");
		return;
	}
	switch (obj->type) {
	case ACPI_TYPE_INTEGER:
		lab_printf("%*sout: integer 0x%llx (%llu)\n", depth, "",
			   obj->integer.value, obj->integer.value);
		break;
	case ACPI_TYPE_BUFFER:
		lab_printf("%*sout: buffer len=%u: ", depth, "", obj->buffer.length);
		lab_hex(obj->buffer.pointer, obj->buffer.length);
		lab_printf("\n");
		if (obj->buffer.length == 4)
			lab_printf("%*s     as u32: 0x%08x\n", depth, "",
				   get_unaligned_le32(obj->buffer.pointer));
		else if (obj->buffer.length == 8)
			lab_printf("%*s     as u64: 0x%016llx\n", depth, "",
				   get_unaligned_le64(obj->buffer.pointer));
		break;
	case ACPI_TYPE_STRING:
		lab_printf("%*sout: string \"%s\"\n", depth, "", obj->string.pointer);
		break;
	case ACPI_TYPE_PACKAGE:
		lab_printf("%*sout: package count=%u\n", depth, "", obj->package.count);
		for (i = 0; i < obj->package.count; i++)
			lab_dump_object(&obj->package.elements[i], depth + 2);
		break;
	default:
		lab_printf("%*sout: object type %u\n", depth, "", obj->type);
	}
}

static ssize_t call_store(struct device *dev, struct device_attribute *attr,
			  const char *buf, size_t count)
{
	struct acpi_buffer output = { ACPI_ALLOCATE_BUFFER, NULL };
	struct acpi_buffer input;
	char *kbuf, *p, *guid, *mid_s, *hex;
	u32 method_id;
	u8 instance = 0;
	u8 *in;
	int in_len, ret = 0;
	acpi_status status;

	kbuf = kstrndup(buf, count, GFP_KERNEL);
	if (!kbuf)
		return -ENOMEM;

	p = skip_spaces(kbuf);
	guid = strsep(&p, " \t\n");
	mid_s = p ? strsep(&p, " \t\n") : NULL;
	hex = p ? p : "";
	if (!guid || !*guid || !mid_s || !*mid_s ||
	    kstrtou32(mid_s, 0, &method_id)) {
		ret = -EINVAL;
		goto out_free;
	}

	in = kzalloc(LAB_MAX_IN, GFP_KERNEL);
	if (!in) {
		ret = -ENOMEM;
		goto out_free;
	}
	in_len = lab_parse_hex(hex, in, LAB_MAX_IN);
	if (in_len < 0) {
		ret = in_len;
		goto out_free_in;
	}
	if (in_len == 0)
		in_len = 4;	/* zeroed dummy argument */
	input.length = in_len;
	input.pointer = in;

	mutex_lock(&lab_lock);
	lab_result_len = 0;
	lab_result[0] = '\0';
	lab_printf("guid %s method %u instance %u\nin  : len=%d: ",
		   guid, method_id, instance, in_len);
	lab_hex(in, in_len);
	lab_printf("\n");

	if (!wmi_has_guid(guid)) {
		lab_printf("status: GUID not present on this system\n");
		ret = -ENODEV;
		goto out_unlock;
	}

	status = wmi_evaluate_method(guid, instance, method_id, &input, &output);
	lab_printf("status: 0x%x (%s)\n", status, acpi_format_exception(status));
	if (ACPI_SUCCESS(status))
		lab_dump_object(output.pointer, 0);
	kfree(output.pointer);

out_unlock:
	mutex_unlock(&lab_lock);
out_free_in:
	kfree(in);
out_free:
	kfree(kbuf);
	return ret ? ret : count;
}

static ssize_t call_show(struct device *dev, struct device_attribute *attr,
			 char *buf)
{
	ssize_t n;

	mutex_lock(&lab_lock);
	n = scnprintf(buf, PAGE_SIZE, "%s", lab_result);
	mutex_unlock(&lab_lock);
	return n;
}

static DEVICE_ATTR(call, 0600, call_show, call_store);

static int __init lab_init(void)
{
	int ret;

	lab_result = kzalloc(LAB_MAX_RESULT, GFP_KERNEL);
	if (!lab_result)
		return -ENOMEM;

	lab_pdev = platform_device_register_simple("acer-wmi-lab",
						   PLATFORM_DEVID_NONE, NULL, 0);
	if (IS_ERR(lab_pdev)) {
		ret = PTR_ERR(lab_pdev);
		goto err_free;
	}
	ret = device_create_file(&lab_pdev->dev, &dev_attr_call);
	if (ret)
		goto err_unregister;

	pr_info("acer-wmi-lab: ready at /sys/devices/platform/acer-wmi-lab/call\n");
	return 0;

err_unregister:
	platform_device_unregister(lab_pdev);
err_free:
	kfree(lab_result);
	return ret;
}

static void __exit lab_exit(void)
{
	device_remove_file(&lab_pdev->dev, &dev_attr_call);
	platform_device_unregister(lab_pdev);
	kfree(lab_result);
}

module_init(lab_init);
module_exit(lab_exit);

MODULE_AUTHOR("Fernando Klumpp");
MODULE_DESCRIPTION("Research helper to evaluate Acer ACPI-WMI methods from user space");
MODULE_LICENSE("GPL");
