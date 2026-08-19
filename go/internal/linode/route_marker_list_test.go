package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The one route on the surface declaring the marker envelope, and the bucket
// every case here addresses.
const (
	bucketObjectListTool = "linode_object_storage_bucket_object_list"
	bucketRegion         = "us-east-1"
	bucketLabel          = "photos"
)

// The S3-backed body: elements under data, plus the cursor beside them. Every
// case here serves this shape or a variation of it.
const truncatedBucketBody = `{"data":[{"name":"images/a.png"},{"name":"images/b.png"}],` +
	`"is_truncated":true,"next_marker":"images/b.png"}`

func TestListProtoRouteMarkerCarriesTheCursorBackWithTheElements(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		if _, err := w.Write([]byte(truncatedBucketBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	items, cursor, err := linode.ListProtoRouteMarker(t.Context(), client, bucketObjectListTool,
		[]any{bucketRegion, bucketLabel}, "prefix=images%2F",
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
	if err != nil {
		t.Fatalf("ListProtoRouteMarker: %v", err)
	}

	if want := "/object-storage/buckets/us-east-1/photos/object-list"; !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want it to end with %q", gotPath, want)
	}

	if len(items) != 2 || items[1].GetName() != "images/b.png" {
		t.Fatalf("items = %v, want the two decoded objects", items)
	}

	if !cursor.IsTruncated || cursor.NextMarker != "images/b.png" {
		t.Errorf("cursor = %+v, want the truncated page's resume point", cursor)
	}
}

// A complete listing answers without the cursor members at all, which reads as
// the page that is not truncated rather than as a malformed body.
func TestListProtoRouteMarkerReadsAnAbsentCursorAsAWholeCollection(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"data":[{"name":"only.png"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	items, cursor, err := linode.ListProtoRouteMarker(t.Context(),
		newRouteTestClient(t, server.URL), bucketObjectListTool,
		[]any{bucketRegion, bucketLabel}, "",
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
	if err != nil {
		t.Fatalf("ListProtoRouteMarker: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("items = %v, want the one decoded object", items)
	}

	if cursor.IsTruncated || cursor.NextMarker != "" {
		t.Errorf("cursor = %+v, want no resume point", cursor)
	}
}

// A cursor member of the wrong JSON type is a shape change upstream, which has
// to fail loudly: read as a zero it would report a truncated page as complete.
func TestListProtoRouteMarkerRefusesACursorItCannotDecode(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "truncated is not a bool", body: `{"data":[],"is_truncated":"yes"}`},
		{name: "marker is not text", body: `{"data":[],"next_marker":42}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			_, _, err := linode.ListProtoRouteMarker(t.Context(),
				newRouteTestClient(t, server.URL), bucketObjectListTool,
				[]any{bucketRegion, bucketLabel}, "",
				func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
			if err == nil {
				t.Fatal("ListProtoRouteMarker succeeded, want a decode failure")
			}
		})
	}
}

// The primitives are picked by the declared envelope, so reaching for the wrong
// one has to fail rather than answer: the page reader would drop the cursor,
// and the marker reader would invent one for a page that carries none.
func TestTheListPrimitivesRefuseAnEnvelopeTheyDoNotServe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(truncatedBucketBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := newRouteTestClient(t, server.URL)

	if _, err := linode.ListProtoRoute(t.Context(), client, bucketObjectListTool,
		[]any{bucketRegion, bucketLabel}, "", 0, 0,
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} },
	); err == nil {
		t.Error("ListProtoRoute served a marker-paged route, want a refusal")
	}

	_, _, err := linode.ListProtoRouteMarker(t.Context(), client, domainListTool, nil, "",
		func() *linodev1.Domain { return &linodev1.Domain{} })
	if !errors.Is(err, linode.ErrShapeMismatch) {
		t.Errorf("error = %v, want ErrShapeMismatch refusing the page-envelope route", err)
	}
}

// A body that is not an object cannot carry a cursor, and decoding it as one
// would answer a real failure as an empty untruncated page.
func TestListProtoRouteMarkerHoldsTheBodyToBeingAnObject(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`[{"name":"a.png"}]`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, _, err := linode.ListProtoRouteMarker(t.Context(),
		newRouteTestClient(t, server.URL), bucketObjectListTool,
		[]any{bucketRegion, bucketLabel}, "",
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
	if err == nil {
		t.Fatal("ListProtoRouteMarker succeeded, want a refusal")
	}
}

// The elements are held to the same shapes every other list shape holds them
// to, so a body whose data member is not an array of objects fails here rather
// than reaching the caller as an empty page beside a real cursor.
func TestListProtoRouteMarkerRefusesElementsItCannotDecode(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "data is not an array", body: `{"data":{"name":"a.png"},"is_truncated":false}`},
		{name: "an element is not an object", body: `{"data":["a.png"],"is_truncated":false}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(test.body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			_, _, err := linode.ListProtoRouteMarker(t.Context(),
				newRouteTestClient(t, server.URL), bucketObjectListTool,
				[]any{bucketRegion, bucketLabel}, "",
				func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
			if err == nil {
				t.Fatal("ListProtoRouteMarker succeeded, want a decode failure")
			}
		})
	}
}

// The path values fill the route template, so a wrong count addresses something
// other than the bucket the caller named.
func TestListProtoRouteMarkerRefusesAWrongPathValueCount(t *testing.T) {
	t.Parallel()

	_, _, err := linode.ListProtoRouteMarker(t.Context(),
		newRouteTestClient(t, "http://127.0.0.1:1"), bucketObjectListTool,
		[]any{bucketRegion}, "",
		func() *linodev1.ObjectStorageObject { return &linodev1.ObjectStorageObject{} })
	if err == nil {
		t.Fatal("ListProtoRouteMarker succeeded with one path value, want a refusal")
	}
}
