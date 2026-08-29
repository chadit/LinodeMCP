package main

import "slices"

// The two directions a record shape carries beyond its projection: the
// canonical bytes an engine writes to disk, and the value it reads back.
//
// A projection answers a plain body whose member order nothing downstream
// depends on, because the body is re-serialized against its message. A RECORD is
// the bytes themselves, so its member order is the format. Emitting both
// directions from the same member list is what stops one language writing the
// audit log in its struct's own field order while another writes it in the
// contract's.
//
// What each language spells for itself is the value encoder it already has.
// The order, the member set and the readers are the contract's.

// recordShapes is every shape declaring local_record, in the order the shapes
// were placed, so both languages emit the same sequence.
func recordShapes(shapes []answerShape) []*answerShape {
	records := make([]*answerShape, 0, len(shapes))

	for index := range shapes {
		if shapes[index].record {
			records = append(records, &shapes[index])
		}
	}

	return records
}

// The helper names the two record directions reach for. The writer's three are
// pulled in by any record at all; the readers are pulled in per member, so a
// language emits only the ones some record's member is read through.
const (
	recordHelperJSON   = "recordJSON"
	recordHelperValue  = "RecordValue"
	recordHelperFields = "recordFields"
)

// recordWriterHelpers is what any record at all pulls in: the member list
// writer, the per-member encoder it calls, and the decode the reader opens with.
func recordWriterHelpers() []string {
	return []string{recordHelperJSON, recordHelperValue, recordHelperFields}
}

// recordHelpersUsed is every helper the emitted record directions reach, in a
// settled order: the writer's own, then one reader per member kind some record
// carries. A helper nothing reads is left out, since the tree it lands in is
// linted for dead code the same way hand-written source is.
func recordHelpersUsed(shapes []answerShape, reader func(*answerMember) string) []string {
	records := recordShapes(shapes)
	if len(records) == 0 {
		return nil
	}

	used := recordWriterHelpers()

	for _, shape := range records {
		for index := range shape.members {
			name := reader(&shape.members[index])
			if !slices.Contains(used, name) {
				used = append(used, name)
			}
		}
	}

	return used
}
