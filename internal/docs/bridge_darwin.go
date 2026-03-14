//go:build darwin && cgo

package docs

/*
#cgo darwin CFLAGS: -fobjc-arc
#cgo darwin LDFLAGS: -framework Foundation
#include <stdbool.h>
#include <stdlib.h>

int doq_docs_available(char **err_out);
char *doq_docs_search_json(const char *query, const char *frameworks_json, const char *kinds_json, int limit, bool omit_content, char **err_out);
char *doq_docs_get_json(const char *identifier, char **err_out);
void doq_docs_free(char *ptr);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

func available() error {
	var errOut *C.char
	if C.doq_docs_available(&errOut) == 1 {
		return nil
	}
	return bridgeError(errOut)
}

func searchJSON(query string, opts SearchOptions) ([]byte, error) {
	if err := available(); err != nil {
		return nil, err
	}

	frameworksJSON, err := json.Marshal(opts.Frameworks)
	if err != nil {
		return nil, fmt.Errorf("encoding docs search frameworks: %w", err)
	}
	kindsJSON, err := json.Marshal(opts.Kinds)
	if err != nil {
		return nil, fmt.Errorf("encoding docs search kinds: %w", err)
	}

	cQuery := C.CString(query)
	cFrameworks := C.CString(string(frameworksJSON))
	cKinds := C.CString(string(kindsJSON))
	defer C.free(unsafe.Pointer(cQuery))
	defer C.free(unsafe.Pointer(cFrameworks))
	defer C.free(unsafe.Pointer(cKinds))

	var errOut *C.char
	ptr := C.doq_docs_search_json(cQuery, cFrameworks, cKinds, C.int(opts.Limit), C.bool(opts.OmitContent), &errOut)
	if ptr == nil {
		return nil, bridgeError(errOut)
	}
	defer C.doq_docs_free(ptr)

	return []byte(C.GoString(ptr)), nil
}

func getJSON(identifier string) ([]byte, error) {
	if err := available(); err != nil {
		return nil, err
	}

	cIdentifier := C.CString(identifier)
	defer C.free(unsafe.Pointer(cIdentifier))

	var errOut *C.char
	ptr := C.doq_docs_get_json(cIdentifier, &errOut)
	if ptr == nil {
		return nil, bridgeError(errOut)
	}
	defer C.doq_docs_free(ptr)

	return []byte(C.GoString(ptr)), nil
}

func bridgeError(errOut *C.char) error {
	if errOut == nil {
		return ErrUnavailable
	}
	defer C.doq_docs_free(errOut)

	msg := C.GoString(errOut)
	if msg == "" {
		return ErrUnavailable
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, msg)
}
