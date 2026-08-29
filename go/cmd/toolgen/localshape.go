package main

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// How the shape an engine operation fills is held against the response a tool
// declares.
//
// An operation answers a plain body naming no message, which is what lets two
// tools share one. The cost of that is that nothing in the operation says which
// messages the body fits, so the table states the shape and this compares it
// with each declaring tool's response. The comparison is by member rather than
// by name: two differently named messages carrying one member set under one set
// of kinds are the same shape, which is exactly the case that lets a tool
// declare its own response over a shared operation.

// localShapeDifference is how the message a tool declares differs from the one
// its operation fills, "" where the two are the same shape.
//
// Both descriptors are handed in rather than resolved by name. The arm table
// names generated types and a declared response is resolved once when the
// contract reads it, so nothing here reaches the process-global registry: a
// caller cannot take its read lock while another goroutine holds it across a
// range, which is a deadlock the parallel emitter tests can reach.
func localShapeDifference(filled, declared protoreflect.MessageDescriptor) string {
	return localMessageDifference(filled, declared, nil)
}

// localMessageDifference compares two messages member by member, answering the
// first difference it finds.
//
// Both descriptors are always present: the arm table states its shapes as
// generated messages and a declared response is resolved when the contract
// reads it, so there is no unresolved name to answer for.
//
// seen carries the pairs already under comparison, which is what ends the walk
// where two messages reach each other again. The contract holds messages that
// reach themselves, so without it a member of its own type never terminates.
func localMessageDifference(filled, declared protoreflect.MessageDescriptor, seen []string) string {
	pair := string(filled.FullName()) + " " + string(declared.FullName())
	if slices.Contains(seen, pair) {
		return ""
	}

	if difference := localMemberSets(filled.Fields(), declared.Fields()); difference != "" {
		return difference
	}

	return localMemberKinds(filled.Fields(), declared.Fields(), append(seen, pair))
}

// localMemberSets holds the two member sets to each other by name, in each
// message's own declaration order so one difference reports the same way on
// every run.
func localMemberSets(filled, declared protoreflect.FieldDescriptors) string {
	for index := range filled.Len() {
		member := filled.Get(index)
		if declared.ByName(member.Name()) == nil {
			return fmt.Sprintf("fills %s, which it does not declare", member.Name())
		}
	}

	for index := range declared.Len() {
		member := declared.Get(index)
		if filled.ByName(member.Name()) == nil {
			return fmt.Sprintf("fills no %s", member.Name())
		}
	}

	return ""
}

// localMemberKinds holds each member the two share to one type, which is what
// stops a list arriving where one value is declared and a number where text is.
func localMemberKinds(filled, declared protoreflect.FieldDescriptors, seen []string) string {
	for index := range filled.Len() {
		member := filled.Get(index)

		difference := localMemberDifference(member, declared.ByName(member.Name()), seen)
		if difference != "" {
			return difference
		}
	}

	return ""
}

// localMemberDifference is how one member of the filled shape differs from the
// member the response declares under that name.
//
// Two enum members of the same form are read as agreeing. The contract carries
// no member name declared over two different enums, so a check for it would be
// a branch nothing reaches.
func localMemberDifference(filled, declared protoreflect.FieldDescriptor, seen []string) string {
	if localMemberForm(filled) != localMemberForm(declared) {
		return fmt.Sprintf("fills %s as %s and it declares %s",
			filled.Name(), localMemberForm(filled), localMemberForm(declared))
	}

	if filled.Message() == nil {
		return ""
	}

	return localMessageDifference(filled.Message(), declared.Message(), seen)
}

// localMemberForm is one member's type as a reader reads it: its kind, and
// whether it holds many of them. A map reports its value's kind, since that is
// what a body has to carry under each key.
func localMemberForm(member protoreflect.FieldDescriptor) string {
	if member.IsMap() {
		return "a map of " + member.MapValue().Kind().String()
	}

	if member.IsList() {
		return "repeated " + member.Kind().String()
	}

	return member.Kind().String()
}
