package export

import "github.com/google/uuid"

var lastUUID uuid.UUID

func quickUUID() string {
	if lastUUID == uuid.Nil {
		lastUUID = uuid.New()
		return lastUUID.String()
	}
	lastUUID[0]++
	lastUUID[2]++
	lastUUID[4]++
	lastUUID[6] = (lastUUID[6] & 0x0f) | 0x40 // Version 4
	lastUUID[8] = (lastUUID[8] & 0x3f) | 0x80 // Variant is 10
	return lastUUID.String()
}

func stringExample(format *string) *string {
	if format == nil {
		return nil
	}
	switch *format {
	case "uuid":
		return Ptr(quickUUID())
	case "email":
		return Ptr("test@example.com")
	case "hostname":
		return Ptr("example.com")
	case "ipv4":
		return Ptr("10.10.10.10")
	case "ipv6":
		return Ptr("2001:db8::68")
	case "uri":
		return Ptr("https://example.com")
	case "date":
		return Ptr("2021-01-01")
	case "number":
		return Ptr("12.34")
	default:
		return nil
	}
}
