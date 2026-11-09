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

// FirstIE returns the first state registration (IE) of person.state_registrations, or "".
func FirstIE(person map[string]any) string {
	if regs, ok := person["state_registrations"].([]any); ok && len(regs) > 0 {
		if rm, ok := regs[0].(map[string]any); ok {
			if v, ok := rm["state_registration"].(string); ok {
				return v
			}
		}
	}
	return ""
}

// IEForUF returns the IE registered for uf only (no fallback to another UF), digits only. "ISENTO" or
// punctuation-only values yield "", because the MDF-e XSD TIe is [0-9]{2,14}.
func IEForUF(person map[string]any, uf string) string {
	regs, _ := person["state_registrations"].([]any)
	for _, r := range regs {
		rm, ok := r.(map[string]any)
		if !ok || rm["uf"] != uf {
			continue
		}
		v, _ := rm["state_registration"].(string)
		var b []byte
		for i := 0; i < len(v); i++ {
			if v[i] >= '0' && v[i] <= '9' {
				b = append(b, v[i])
			}
		}
		if len(b) >= 2 && len(b) <= 14 {
			return string(b)
		}
	}
	return ""
}
