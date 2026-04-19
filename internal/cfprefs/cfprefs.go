// Package cfprefs provides a CGO wrapper around the macOS CoreFoundation
// CFPreferencesCopyAppValue API. It reads a preference key from a given
// domain, respecting the MDM-managed layer, and returns the value serialized
// as an XML plist byte slice suitable for decoding with howett.net/plist.
package cfprefs

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

// PropertyListToXMLData serializes any CFPropertyListRef to an XML plist.
CFDataRef PropertyListToXMLData(CFPropertyListRef propList) {
    if (!propList) return NULL;
    return CFPropertyListCreateData(
        kCFAllocatorDefault,
        propList,
        kCFPropertyListXMLFormat_v1_0,
        0,
        NULL
    );
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// CopyAppValue reads the value for key in domain from the macOS preference
// system. It respects the MDM "Managed" layer — if the key is enforced by
// MDM it will be returned even if no local user preference exists.
//
// Returns the value serialized as an XML plist []byte, or an error if the
// key is not present or cannot be serialized.
func CopyAppValue(domain, key string) ([]byte, error) {
	cDomain := C.CFStringCreateWithCString(C.kCFAllocatorDefault, C.CString(domain), C.kCFStringEncodingUTF8)
	cKey := C.CFStringCreateWithCString(C.kCFAllocatorDefault, C.CString(key), C.kCFStringEncodingUTF8)
	defer C.CFRelease(C.CFTypeRef(cDomain))
	defer C.CFRelease(C.CFTypeRef(cKey))

	propList := C.CFPreferencesCopyAppValue(cKey, cDomain)
	if propList == 0 {
		return nil, fmt.Errorf("key %q not found in domain %q", key, domain)
	}
	defer C.CFRelease(propList)

	xmlData := C.PropertyListToXMLData(propList)
	if xmlData == 0 {
		return nil, fmt.Errorf("failed to serialize preference value to XML plist")
	}
	defer C.CFRelease(C.CFTypeRef(xmlData))

	ptr := C.CFDataGetBytePtr(xmlData)
	length := C.CFDataGetLength(xmlData)
	return C.GoBytes(unsafe.Pointer(ptr), C.int(length)), nil
}
