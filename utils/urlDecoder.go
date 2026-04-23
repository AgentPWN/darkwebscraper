package utils

import "net/url"

func URLDecode(input string) (string, error) {
	decoded, err := url.QueryUnescape(input)
	if err != nil {
		return "", err
	}
	return decoded, nil
}
