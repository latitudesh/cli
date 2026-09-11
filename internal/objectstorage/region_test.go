package objectstorage

import "testing"

func TestSigningRegion(t *testing.T) {
	cases := map[string]string{
		"https://s3.us-east-1.storage.sh":      "us-east-1",
		"https://s3.eu-west-2.storage.sh":      "eu-west-2",
		"https://s3.ap-northeast-1.storage.sh": "ap-northeast-1",
		"https://objects.tyo.storage.sh":       "tyo",
		"https://objects.nyc.storage.sh":       "nyc",
		"objects.lon2.storage.sh":              "lon2",
		"https://127.0.0.1:9000":               DefaultSigningRegion,
		"http://localhost:9000":                DefaultSigningRegion,
		"https://example.com":                  DefaultSigningRegion,
		"":                                     DefaultSigningRegion,
	}
	for in, want := range cases {
		if got := SigningRegion(in); got != want {
			t.Errorf("SigningRegion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEndpointHost(t *testing.T) {
	host, secure, err := EndpointHost("https://s3.us-east-1.storage.sh")
	if err != nil || host != "s3.us-east-1.storage.sh" || !secure {
		t.Errorf("https: %q %v %v", host, secure, err)
	}
	host, secure, err = EndpointHost("http://127.0.0.1:9000")
	if err != nil || host != "127.0.0.1:9000" || secure {
		t.Errorf("http: %q %v %v", host, secure, err)
	}
	host, secure, err = EndpointHost("objects.tyo.storage.sh")
	if err != nil || host != "objects.tyo.storage.sh" || !secure {
		t.Errorf("bare: %q %v %v", host, secure, err)
	}
	if _, _, err := EndpointHost("ftp://x"); err == nil {
		t.Error("ftp should be rejected")
	}
}
