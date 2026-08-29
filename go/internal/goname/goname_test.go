package goname_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/goname"
)

// TestExportedSpellsInitialisms covers the reason this table is shared: the
// emitter writes the factory name, the handler name and every call between them
// through it, so a word rendered two ways is a call to a function nobody
// wrote.
func TestExportedSpellsInitialisms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain words", in: "linode_domain_record_get", want: "LinodeDomainRecordGet"},
		{name: "one word", in: "hello", want: "Hello"},
		{name: "acl", in: "linode_object_storage_object_acl_get", want: "LinodeObjectStorageObjectACLGet"},
		{name: "two initialisms", in: "linode_vpc_ip_list", want: "LinodeVPCIPList"},
		{name: "ssl", in: "linode_database_mysql_instance_ssl_get", want: "LinodeDatabaseMysqlInstanceSSLGet"},
		{name: "id", in: "domain_id", want: "DomainID"},
		{name: "url", in: "presigned_url_create", want: "PresignedURLCreate"},
		{name: "uuid", in: "token_uuid", want: "TokenUUID"},
		{name: "ipv6", in: "linode_ipv6_range_get", want: "LinodeIPv6RangeGet"},
		{name: "ipv4", in: "reserved_ipv4_addresses", want: "ReservedIPv4Addresses"},
		// lke is deliberately absent from the table: revive's var-naming reads
		// Lke as an ordinary word, and only the words it would reject belong.
		{name: "api", in: "lke_api_endpoint_list", want: "LkeAPIEndpointList"},
		{name: "hyphen separated", in: "fetch-state", want: "FetchState"},
		// A doubled separator splits to an empty word, and capitalizing one
		// indexes past its end. No manifest name carries one, which is the
		// point: a name that never should reach here must not panic if it does.
		{name: "doubled separator", in: "linode__odd_tool", want: "LinodeOddTool"},
		{name: "empty", in: "", want: ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := goname.Exported(testCase.in); got != testCase.want {
				t.Errorf("Exported(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}

// TestInitialismAnswersOnlyKnownWords guards the half Exported cannot show: a
// word that is not an initialism has to report that it is not one, or every
// caller would have to know the table's contents to use it.
func TestInitialismAnswersOnlyKnownWords(t *testing.T) {
	t.Parallel()

	known := map[string]string{
		"id": "ID", "ip": "IP", "ipv4": "IPv4", "ipv6": "IPv6",
		"url": "URL", "api": "API", "acl": "ACL", "uuid": "UUID",
		"ssl": "SSL", "vpc": "VPC",
	}

	for word, want := range known {
		got, ok := goname.Initialism(word)
		if !ok || got != want {
			t.Errorf("Initialism(%q) = %q, %t; want %q, true", word, got, ok, want)
		}
	}

	for _, word := range []string{"linode", "ids", "IP", "vpcs", "", "acl_get"} {
		if got, ok := goname.Initialism(word); ok {
			t.Errorf("Initialism(%q) = %q, true; want not an initialism", word, got)
		}
	}
}
