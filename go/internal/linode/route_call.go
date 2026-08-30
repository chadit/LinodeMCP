package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// These primitives take the tool name and response message as arguments, so
// cmd/toolgen can emit a call for any tool the contract declares without a
// matching client method existing first. The retry loop's operation label is
// the tool name because a generated call site has no verb-and-noun to read,
// and inventing one would put a fact in the generator that nothing else can
// check.

// CallProtoRouteQuery performs the route the named tool declares and decodes
// the response into msg. pathValues fill the route template's slots in
// declared order. rawQuery is already encoded, the same form the list
// primitives take, and carries what a single-resource route also reads from
// its query: the page controls /databases/types/{id} publishes, or the object
// key /object-storage/buckets/{r}/{l}/object-acl is addressed by.
//
// msg is reset per attempt because protojson merges into a message rather than
// replacing it, so a retried attempt over a partly-filled msg would blend two
// responses.
func (c *Client) CallProtoRouteQuery(
	ctx context.Context, tool string, pathValues []any, rawQuery string, msg proto.Message,
) error {
	return c.executeWithRetry(ctx, tool, func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestQuery(attemptCtx, tool, rawQuery, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		_, err = c.handleProtoResponseSubject(resp, routeSubject(tool), msg)

		return err
	})
}

// routeSubject names a read in the report its answer gets when the API sends
// something that is not a JSON object. A decoder's own complaint cannot say
// which call answered badly, and Python refuses the same body with the same
// sentence, so one fixture covers both.
//
// The prose is derived rather than declared, the way cmd/toolgen derives a
// mutation's subject from the same tool name:
// linode_networking_reserved_ip_get reads as "networking reserved ip get".
func routeSubject(tool string) string {
	return strings.ReplaceAll(strings.TrimPrefix(tool, "linode_"), "_", " ")
}

// CallProtoRouteQueryRaw is CallProtoRouteQuery for the reads whose answer
// restores what the decode drops: it hands back the body it decoded, so the
// serializer can write the documented explicit nulls protojson emits no member
// for, the same way the mutating primitives do.
func (c *Client) CallProtoRouteQueryRaw(
	ctx context.Context, tool string, pathValues []any, rawQuery string, msg proto.Message,
) (json.RawMessage, error) {
	var body json.RawMessage

	err := c.executeWithRetry(ctx, tool, func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestQuery(attemptCtx, tool, rawQuery, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		body, err = c.handleProtoResponseSubject(resp, routeSubject(tool), msg)

		return err
	})

	return body, err
}

// CallProtoRouteState performs the read a removal declares its state fetch
// through, decoding the resource and handing back the raw object the decode
// read so the caller can restore the keys the API sent as an explicit null.
//
// member names the key the resource sits under, "" when the body is the
// resource itself. The control-plane ACL route wraps its resource that way, and
// decoding the whole body would answer an empty message, since the wrapper's
// own members are not the resource's.
func (c *Client) CallProtoRouteState(
	ctx context.Context, tool string, pathValues []any, member string, msg proto.Message,
) (json.RawMessage, error) {
	return c.CallProtoRouteStateQuery(ctx, tool, pathValues, "", member, msg)
}

// CallProtoRouteStateQuery is the same read for the routes a query addresses as
// well as a path: the object ACL names its bucket in the path and its object in
// the query, so the resource is not reachable without both.
func (c *Client) CallProtoRouteStateQuery(
	ctx context.Context, tool string, pathValues []any, rawQuery, member string, msg proto.Message,
) (json.RawMessage, error) {
	if member == "" {
		return c.CallProtoRouteQueryRaw(ctx, tool, pathValues, rawQuery, msg)
	}

	// The envelope decodes into a free-form Struct so the status, shape and
	// retry handling every other read gets applies here too; only the member
	// below is decoded into the resource's own message.
	var envelope structpb.Struct

	body, err := c.CallProtoRouteQueryRaw(ctx, tool, pathValues, rawQuery, &envelope)
	if err != nil {
		return nil, err
	}

	return decodeStateMember(body, tool, member, msg)
}

// decodeStateMember decodes the member of a raw answer the resource sits under,
// answering that member's own bytes so a null restored into it lands beside the
// resource's fields rather than the envelope's.
func decodeStateMember(
	body json.RawMessage, tool, member string, msg proto.Message,
) (json.RawMessage, error) {
	var fields map[string]json.RawMessage

	// The shared response handler proved the body is a JSON object before it
	// handed the bytes back, so a decode cannot fail here; a member the answer
	// does not carry is the only way this read comes back without a resource.
	if err := json.Unmarshal(body, &fields); err != nil || fields[member] == nil {
		return nil, fmt.Errorf("%s %w: %s", routeSubject(tool), ErrStateMemberMissing, member)
	}

	resource := fields[member]

	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(resource, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proto response: %w", err)
	}

	return resource, nil
}

// CallRouteObject performs the route the named tool declares and decodes the
// answer as a free-form JSON object. It is the read primitive for the routes
// whose body has no proto model: the database engine configs, profile
// preferences, managed stats. There is no message to decode into, so the map is
// what the serializer sorts through a bare Struct.
//
// The map is cleared per attempt for the reason CallProtoRouteQuery resets
// its message: json.Unmarshal merges into a map rather than replacing it, so a
// retried attempt would blend two responses.
func (c *Client) CallRouteObject(
	ctx context.Context, tool string, pathValues []any, rawQuery string,
) (map[string]any, error) {
	var object map[string]any

	err := c.executeWithRetry(ctx, tool, func() error {
		object = nil

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestQuery(attemptCtx, tool, rawQuery, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, &object)
	})

	return object, err
}

// CallRouteJSON performs the read the named tool declares and decodes the JSON
// answer into out, for the caller whose model of the answer is a hand-written
// struct rather than a proto message: the scope validator reads the profile
// and its grants into the types its grant flattening walks.
//
// Nothing resets out between attempts. A decode failure is not retried, and
// every failure that is retried happens before the decode, so a partly-filled
// out never meets a second answer.
func (c *Client) CallRouteJSON(ctx context.Context, tool string, pathValues []any, out any) error {
	return c.executeWithRetry(ctx, tool, func() error {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, out)
	})
}

// CallProtoRouteBody performs the route the named tool declares with body as
// the JSON request body, decoding the response into msg. It is
// CallProtoRouteQuery for the mutating tiers, which differ from a read in two ways and no others:
// they send something, and some of them must not be sent twice.
//
// The retry policy comes from the contract rather than from the caller. A
// create whose id the API assigns cannot be replayed after a timeout without
// leaving a second resource nobody hears about, and letting a call site choose
// would put that judgment in as many places as there are call sites.
//
// msg is reset per attempt for the reason CallProtoRouteQuery documents: protojson
// merges into a message rather than replacing it.
//
// subject names the call in the report when the API answers with something
// other than a JSON object, so a caller hears which mutation went wrong rather
// than what the decoder made of the bytes.
func (c *Client) CallProtoRouteBody(
	ctx context.Context, tool string, pathValues []any, body any, subject string, msg proto.Message,
) error {
	attempt := func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		_, err = c.handleProtoResponseSubject(resp, subject, msg)

		return err
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// CallProtoRouteBodyQuery is CallProtoRouteBody for a mutation whose route also
// takes query parameters: the page controls the two firewall replacements
// publish. rawQuery is already encoded, the same form the list primitives take.
func (c *Client) CallProtoRouteBodyQuery(
	ctx context.Context, tool string, pathValues []any, rawQuery string,
	body any, subject string, msg proto.Message,
) error {
	attempt := func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequestQuery(attemptCtx, tool, rawQuery, body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		_, err = c.handleProtoResponseSubject(resp, subject, msg)

		return err
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// attemptUnderPolicy runs a mutation's attempt under the retry policy its
// contract declares, which is the one rule every mutating primitive shares.
func (c *Client) attemptUnderPolicy(ctx context.Context, tool string, attempt func() error) error {
	if linoderoute.RetryDisabled(tool) {
		return c.executeWithoutRetry(ctx, tool, attempt)
	}

	return c.executeWithRetry(ctx, tool, attempt)
}

// ListProtoRouteBody performs the mutation the named tool declares with body as
// the JSON request body and decodes the page of elements its route answers
// with, rather than one resource.
//
// Both firewall replacements are the shape: the PUT reports every assignment
// that now exists, and the API sends them in the {data, page, pages, results}
// envelope its list route uses. Decoding that into the tool's own response
// message instead places nothing, so the caller reads an empty collection over
// a replacement that has already happened.
//
// The declared envelope is checked rather than assumed: this primitive reads
// the standard page, and a route answering any other shape would decode to
// nothing and report success. cmd/toolgen refuses the declaration ahead of
// here, so reaching this is a contract and generator disagreement.
//
// rawQuery carries the page controls the tool published, already encoded. The
// retry policy is the contract's, the way every other mutating primitive takes
// it.
func ListProtoRouteBody[T proto.Message](
	ctx context.Context,
	client *Client,
	tool string,
	pathValues []any,
	rawQuery string,
	body any,
	newElem func() T,
) ([]T, error) {
	if shape := linoderoute.ListEnvelopeFor(tool).Shape; shape != linoderoute.ListShapeData {
		return nil, fmt.Errorf("%w: %s answers a mutation with a shape other than the standard page", ErrShapeMismatch, tool)
	}

	var items []T

	err := client.attemptUnderPolicy(ctx, tool, func() error {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := client.makeRouteRequestQuery(attemptCtx, tool, rawQuery, body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		items, err = decodeProtoElements(resp, client, tool, newElem)

		return err
	})

	return items, err
}

// CallProtoRouteBodyRaw is CallProtoRouteBody that also answers with the body
// it decoded, for the mutations whose answer restores the explicit nulls the
// decode drops. Retry, reset, and timeout are CallProtoRouteBody's.
func (c *Client) CallProtoRouteBodyRaw(
	ctx context.Context, tool string, pathValues []any, body any, subject string, msg proto.Message,
) (json.RawMessage, error) {
	var raw json.RawMessage

	attempt := func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		raw, err = c.handleProtoResponseSubject(resp, subject, msg)

		return err
	}

	return raw, c.attemptUnderPolicy(ctx, tool, attempt)
}

// CallRoute performs the route the named tool declares with no request body and
// nothing to decode out of the answer, which is the shape of a DELETE: the API
// reports the removal by status alone, and the tool's own response is built
// from the ids it was addressed by.
//
// It follows the same contract-declared retry policy CallProtoRouteBody does.
// A delete is idempotent often enough that most of the surface is safe to
// replay, but the destroy tier also covers actions that are not (a rebuild, a
// recycle), and those declare retry_disabled rather than each call site
// deciding.
func (c *Client) CallRoute(ctx context.Context, tool string, pathValues []any) error {
	attempt := func() error {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, nil)
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// CallRouteBody performs the route the named tool declares with body as the
// JSON request body and nothing to decode out of the answer. It is CallRoute
// for the mutations that send something: a phone number to verify, a promo code
// to apply. The API reports the change by status, and the tool's own response is
// built from the arguments it was called with.
//
// It follows the same contract-declared retry policy CallProtoRouteBody does.
func (c *Client) CallRouteBody(ctx context.Context, tool string, pathValues []any, body any) error {
	attempt := func() error {
		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, body, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleResponse(resp, nil)
	}

	return c.attemptUnderPolicy(ctx, tool, attempt)
}

// ListProtoRoute fetches one page of the collection the named tool declares and
// decodes each element into a fresh proto message. pathValues fill the route
// template's slots, so a sub-resource collection (/domains/{domain_id}/records)
// uses the same primitive as a top-level one.
//
// Which JSON shape the body is read as comes from the tool's declared envelope,
// so a route answering a bare array or a member of its own is decoded as what it
// sends. Assuming the standard page for all of them would answer a real
// collection as an empty one and report success.
//
// It is a function rather than a method because Go does not allow a method to
// introduce the type parameter the element type needs.
// rawQuery carries the parameters the route itself filters on, already encoded.
// The page controls are added by the shapes that publish them, so a caller
// hands over only what its own contract declared.
func ListProtoRoute[T proto.Message](
	ctx context.Context,
	client *Client,
	tool string,
	pathValues []any,
	rawQuery string,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	var items []T

	envelope := linoderoute.ListEnvelopeFor(tool)

	err := client.executeWithRetry(ctx, tool, func() error {
		var listErr error

		switch envelope.Shape {
		case linoderoute.ListShapeBare:
			items, listErr = listProtoElementsBareRouted(
				ctx, client, tool, tool, rawQuery, pathValues, newElem,
			)
		case linoderoute.ListShapeSingleton:
			items, listErr = listProtoElementsSingletonRouted(
				ctx, client, tool, tool, rawQuery, pathValues, envelope.Lift, newElem,
			)
		case linoderoute.ListShapeKeyed:
			items, listErr = listProtoElementsKeyedRouted(
				ctx, client, tool, tool, rawQuery, envelope.Member, pathValues, newElem,
			)
		case linoderoute.ListShapeRequiredData:
			items, listErr = listProtoElementsPaginatedRequiredDataRouted(
				ctx, client, tool, tool, rawQuery, pathValues, page, pageSize, newElem,
			)
		case linoderoute.ListShapeData:
			items, listErr = listProtoElementsPaginatedRouted(
				ctx, client, tool, tool, rawQuery, pathValues, page, pageSize, newElem,
			)
		case linoderoute.ListShapeMarker:
			listErr = fmt.Errorf("%w: %s pages by marker", ErrShapeMismatch, tool)
		}

		return listErr
	})

	return items, err
}

// ListProtoRouteRaw is ListProtoRoute for a collection whose elements carry
// documented explicit nulls: it answers each element's body beside the message
// it decoded into, so the serializer can write back the keys protojson emits no
// member for. A page's nulls belong to its elements, which is why the bodies
// come back per element rather than as the one envelope they arrived in.
//
// Only the shapes that read their elements out of an envelope member are
// served. A bare array carries no member, a singleton is one object rather than
// a page, and a marker body pages by a cursor this primitive does not carry
// back, so each answers ErrShapeMismatch rather than a page with no nulls in it.
func ListProtoRouteRaw[T proto.Message](
	ctx context.Context,
	client *Client,
	tool string,
	pathValues []any,
	rawQuery string,
	page, pageSize int,
	newElem func() T,
) ([]T, []json.RawMessage, error) {
	envelope := linoderoute.ListEnvelopeFor(tool)

	var (
		member   = listEnvelopeDataKey
		query    = mergeQuery(rawQuery, pageQuery(page, pageSize))
		required bool
	)

	switch envelope.Shape {
	case linoderoute.ListShapeData:
	case linoderoute.ListShapeRequiredData:
		required = true
	case linoderoute.ListShapeKeyed:
		// A keyed route publishes no page controls: its whole collection
		// arrives under the member it names.
		member, query = envelope.Member, rawQuery
	case linoderoute.ListShapeBare, linoderoute.ListShapeSingleton, linoderoute.ListShapeMarker:
		return nil, nil, fmt.Errorf("%w: %s answers no page member to restore nulls into", ErrShapeMismatch, tool)
	}

	var (
		items []T
		raws  []json.RawMessage
	)

	err := client.executeWithRetry(ctx, tool, func() error {
		var listErr error

		items, raws, listErr = listProtoElementsMemberRaw(
			ctx, client, tool, query, member, pathValues, required, newElem,
		)

		return listErr
	})

	return items, raws, err
}

// ListProtoRouteMarker is ListProtoRoute for the marker shape, whose body
// carries a cursor beside its elements. It is a separate primitive rather than
// a branch of that one because the cursor has to travel back to the caller: a
// truncated page the caller cannot resume from reads as a complete listing.
//
// The page controls take no part here. This route pages by the marker its
// previous answer handed out, and its bound travels as one of the forwarded
// query arguments the caller already encoded into rawQuery.
func ListProtoRouteMarker[T proto.Message](
	ctx context.Context,
	client *Client,
	tool string,
	pathValues []any,
	rawQuery string,
	newElem func() T,
) ([]T, ListMarkerPage, error) {
	if shape := linoderoute.ListEnvelopeFor(tool).Shape; shape != linoderoute.ListShapeMarker {
		return nil, ListMarkerPage{}, fmt.Errorf("%w: %s declares no marker cursor", ErrShapeMismatch, tool)
	}

	var (
		items  []T
		cursor ListMarkerPage
	)

	err := client.executeWithRetry(ctx, tool, func() error {
		var listErr error

		items, cursor, listErr = listProtoElementsMarkerRouted(
			ctx, client, tool, tool, rawQuery, pathValues, newElem,
		)

		return listErr
	})

	return items, cursor, err
}
