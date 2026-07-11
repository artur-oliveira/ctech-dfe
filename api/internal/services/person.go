package services

// Person helpers operate on a person map already decoded from DynamoDB (a
// map[string]any). They are shared by every DFe builder (NF-e, NFC-e, CT-e,
// MDF-e) so the "first address / first phone / first email" lookups are defined
// exactly once. Do not reimplement these inside individual service packages.

// FirstAddress returns the first entry of person.addresses, falling back to the
// legacy singular person.address, or an empty map when neither is present.
func FirstAddress(person map[string]any) map[string]any {
	if addrs, ok := person["addresses"].([]any); ok && len(addrs) > 0 {
		if m, ok := addrs[0].(map[string]any); ok {
			return m
		}
	}
	if addr, ok := person["address"].(map[string]any); ok {
		return addr
	}
	return map[string]any{}
}

// personContacts returns the person.contacts map, or an empty map.
func personContacts(person map[string]any) map[string]any {
	if contacts, ok := person["contacts"].(map[string]any); ok {
		return contacts
	}
	return map[string]any{}
}

// FirstPhone returns the first phone number from person.contacts.phones, or "".
func FirstPhone(person map[string]any) string {
	return firstStringInList(personContacts(person), "phones")
}

// FirstEmail returns the first e-mail from person.contacts.emails, or "".
func FirstEmail(person map[string]any) string {
	return firstStringInList(personContacts(person), "emails")
}

// firstStringInList returns the first string element of contacts[key], or "".
func firstStringInList(contacts map[string]any, key string) string {
	if list, ok := contacts[key].([]any); ok && len(list) > 0 {
		if v, ok := list[0].(string); ok {
			return v
		}
	}
	return ""
}
