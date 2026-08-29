package tools

import (
	"bytes"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// freeFormMessage reports whether a message holds the API's own object rather
// than a modeled shape, so a projection passes its subtree whole.
func freeFormMessage(name protoreflect.FullName) bool {
	switch name {
	case "google.protobuf.Struct", "google.protobuf.Value", "google.protobuf.ListValue":
		return true
	default:
		return false
	}
}

// DeclaredState is the resource a declared fetch read, as the API reported it.
//
// It is a named type rather than the bare map so a dependency walk written for
// a declared fetch cannot be handed a hand-written fetch's own answer, which is
// the pairing that used to fail silently and report no dependencies. Being a
// map underneath is what makes it marshal as the resource itself, which a
// preview reports and a two-stage plan hashes.
type DeclaredState map[string]any

// Number is the whole number the named member carries, and whether it carried
// one: an absent member and one the API sent as null both read as no value.
func (s DeclaredState) Number(name string) (int, bool) {
	number, isNumber := s[name].(json.Number)
	if !isNumber {
		return 0, false
	}

	value, err := number.Int64()
	if err != nil {
		return 0, false
	}

	return int(value), true
}

// Text is the string the named member carries, empty when it carries none.
func (s DeclaredState) Text(name string) string {
	text, _ := s[name].(string)

	return text
}

// Object is the object the named singular member carries, empty when it
// carries none, so a composite state's members read the way a resource does.
func (s DeclaredState) Object(name string) DeclaredState {
	if value, isState := s[name].(DeclaredState); isState {
		return value
	}

	if value, isMap := s[name].(map[string]any); isMap {
		return DeclaredState(value)
	}

	return DeclaredState{}
}

// Objects are the objects the named repeated member carries, each read the way
// the state itself is.
func (s DeclaredState) Objects(name string) []DeclaredState {
	items, isList := s[name].([]any)
	if !isList {
		return nil
	}

	objects := make([]DeclaredState, 0, len(items))

	for _, item := range items {
		// An envelope state's elements arrive already projected, so both the
		// raw map and the projected shape read as one kind of object.
		switch fields := item.(type) {
		case DeclaredState:
			objects = append(objects, fields)
		case map[string]any:
			objects = append(objects, fields)
		}
	}

	return objects
}

// DeclaredStateOf reads the state a declared fetch produced, refusing every
// other shape so a walk paired with a hand-written fetch fails where it can be
// seen rather than reporting an empty walk.
func DeclaredStateOf(state any) (DeclaredState, error) {
	declared, isDeclared := state.(DeclaredState)
	if !isDeclared {
		return nil, ErrStateNotDeclared
	}

	return declared, nil
}

// ProjectDeclaredState is the state a declared fetch reports: the body the API
// sent, carrying the members the read's message models and no others, so a key
// the API sent as null survives and a key it never sent is not invented.
//
// msg is decoded before this runs and is read here for its descriptor alone,
// which is what keeps the reported shape inside the read tool's own contract.
func ProjectDeclaredState(raw json.RawMessage, msg proto.Message) (any, error) {
	body, err := decodeStateBody(raw)
	if err != nil {
		return nil, err
	}

	return DeclaredState(projectFields(body, msg.ProtoReflect().Descriptor())), nil
}

// decodeStateBody reads the API's body keeping every number as it was written,
// so a large id reaches the plan hash the way the API spelled it.
func decodeStateBody(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var body map[string]any

	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrResponseDecode, err)
	}

	if body == nil {
		return nil, fmt.Errorf("%w: body is JSON null", ErrNullMember)
	}

	return body, nil
}

// projectFields keeps the members the message models, reading each under either
// spelling a JSON body can carry it under.
func projectFields(body map[string]any, descriptor protoreflect.MessageDescriptor) map[string]any {
	fields := descriptor.Fields()
	projected := make(map[string]any, fields.Len())

	for index := range fields.Len() {
		field := fields.Get(index)

		value, sent := stateMember(body, field)
		if sent {
			projected[string(field.Name())] = projectMember(value, field)
		}
	}

	return projected
}

// stateMember reads a member the API sent, under the contract's spelling or the
// camel-case one protojson also accepts.
func stateMember(body map[string]any, field protoreflect.FieldDescriptor) (any, bool) {
	if value, sent := body[string(field.Name())]; sent {
		return value, true
	}

	value, sent := body[field.JSONName()]

	return value, sent
}

// projectMember keeps a value as the API sent it, descending only where the
// contract models the shape underneath. A map's keys are the API's rather than
// the contract's, so its subtree passes whole the way a free-form one does.
func projectMember(value any, field protoreflect.FieldDescriptor) any {
	if value == nil || field.Message() == nil || field.IsMap() || freeFormMessage(field.Message().FullName()) {
		return value
	}

	if field.IsList() {
		return projectList(value, field.Message())
	}

	object, isObject := value.(map[string]any)
	if !isObject {
		return value
	}

	return projectFields(object, field.Message())
}

// projectList projects each element the API sent, keeping anything that is not
// an object as it arrived.
func projectList(value any, message protoreflect.MessageDescriptor) any {
	items, isList := value.([]any)
	if !isList {
		return value
	}

	projected := make([]any, 0, len(items))

	for _, item := range items {
		object, isObject := item.(map[string]any)
		if !isObject {
			projected = append(projected, item)

			continue
		}

		projected = append(projected, projectFields(object, message))
	}

	return projected
}
