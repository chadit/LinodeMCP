package tools_test

import "testing"

func shareGroupAssertEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()
	assertEqual(t, expected, actual, msgAndArgs...)
}

func shareGroupAssertTrue(t *testing.T, value bool, msgAndArgs ...any) bool {
	t.Helper()

	return assertTrue(t, value, msgAndArgs...)
}

func shareGroupRequireTrue(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	requireTrue(t, value, msgAndArgs...)
}

func shareGroupAssertFalse(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()
	assertFalse(t, value, msgAndArgs...)
}

func shareGroupRequireFalse(t *testing.T, value bool) {
	t.Helper()
	requireFalse(t, value)
}

func shareGroupRequireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	requireNotNil(t, value, msgAndArgs...)
}

func shareGroupAssertNoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()

	return assertNoError(t, err, msgAndArgs...)
}

func shareGroupRequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	requireNoError(t, err, msgAndArgs...)
}

func shareGroupAssertContains(t *testing.T, collection, contains any, msgAndArgs ...any) {
	t.Helper()
	assertContains(t, collection, contains, msgAndArgs...)
}

func shareGroupAssertNotContains(t *testing.T, collection, contains any, msgAndArgs ...any) {
	t.Helper()
	assertNotContains(t, collection, contains, msgAndArgs...)
}

func shareGroupAssertEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	assertEmpty(t, value, msgAndArgs...)
}

func shareGroupAssertNotEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()
	assertNotEmpty(t, value, msgAndArgs...)
}

func shareGroupAssertLen(t *testing.T, value any, length int, msgAndArgs ...any) bool {
	t.Helper()

	return assertLen(t, value, length, msgAndArgs...)
}

func shareGroupRequireLen(t *testing.T, value any, length int, msgAndArgs ...any) {
	t.Helper()
	requireLen(t, value, length, msgAndArgs...)
}
