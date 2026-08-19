package toolhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// LinodeDomainUpdatePreview fetches the zone as it stands and describes what the
// update would change about it. The request body is deliberately left out of the
// preview: this family's preview predates the body echo, and its shape is what
// clients read today.
func LinodeDomainUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	status := request.GetString("status", "")
	soaEmail := request.GetString("soa_email", "")
	description := request.GetString("description", "")

	return statePreview(ctx, request, cfg, "linode_domain_update", method, path, nil,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetDomain(ctx, domainID)
		},
		func(state any) tools.DryRunDetails {
			return domainUpdateSideEffects(state, status, soaEmail, description)
		})
}

// domainUpdateSideEffects is the Tier B walk for a zone update, diffed against
// the fetched state so the preview says what changes rather than what was asked
// for.
func domainUpdateSideEffects(
	state any, newStatus, newSOA, newDescription string,
) tools.DryRunDetails {
	var (
		details             tools.DryRunDetails
		fromStatus, fromSOA string
	)

	// A fetch that answered with something else leaves both empty, which reads
	// as "no previous value" rather than as no change at all.
	if domain, isDomain := state.(*linode.Domain); isDomain && domain != nil {
		fromStatus, fromSOA = domain.Status, domain.SOAEmail
	}

	if newStatus != "" && newStatus != fromStatus {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Domain status changes from %q to %q.", fromStatus, newStatus))
	}

	if newSOA != "" && newSOA != fromSOA {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("SOA email is set to %q.", newSOA))
	}

	if newDescription != "" {
		details.SideEffects = append(details.SideEffects, "The domain description is updated.")
	}

	return details
}

// LinodeDomainDeleteDependencyWalk names what a zone delete takes with it.
// Every record in the zone goes, so the NS records (the delegation that breaks,
// which is the part a caller cannot put back from memory) are reported one by
// one and the rest are counted in a warning.
//
// A failed record list is a warning rather than an error: the delete is still
// previewable without the record picture, and refusing the preview would leave
// a caller with nothing to decide on.
func LinodeDomainDeleteDependencyWalk(
	ctx context.Context, client *linode.Client, domainID int, _ any,
) (tools.DryRunDetails, error) {
	var details tools.DryRunDetails

	records, err := client.ListDomainRecords(ctx, domainID)
	if err != nil {
		details.Warnings = append(details.Warnings, fmt.Sprintf("Could not list domain records: %v", err))

		return details, nil
	}

	var nsCount int

	for i := range records {
		record := &records[i]
		if !strings.EqualFold(record.Type, "NS") {
			continue
		}

		nsCount++

		details.Dependencies = append(details.Dependencies, tools.DryRunDependency{
			Kind:   "ns_record",
			Label:  record.Target,
			Action: tools.DependencyActionCascadeDeleted,
			Note:   "NS record for " + record.Name,
		})
	}

	if len(records) > 0 {
		details.Warnings = append(details.Warnings, fmt.Sprintf(
			"Deleting this domain destroys %d DNS record(s), including %d NS record(s).", len(records), nsCount,
		))
	}

	return details, nil
}

// LinodeDomainRecordCreatePreview names the record the call would add to the
// zone. A create has no existing record to read, so it reports the request
// alone.
func LinodeDomainRecordCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	recordType := request.GetString("type", "")
	name := request.GetString("name", "")
	target := request.GetString("target", "")

	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_domain_record_create", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_domain_record_create", func() tools.DryRunDetails {
				return domainRecordCreateSideEffects(recordType, name, target, domainID)
			})
		},
	)

	return wrapPreview("linode_domain_record_create", result, err)
}

// domainRecordCreateSideEffects is the Tier B preview prose for a record create.
func domainRecordCreateSideEffects(recordType, name, target string, domainID int) tools.DryRunDetails {
	var details tools.DryRunDetails

	effect := fmt.Sprintf("A new %s record will be created in domain %d", recordType, domainID)
	if name != "" {
		effect += fmt.Sprintf(" for host %q", name)
	}

	if target != "" {
		effect += fmt.Sprintf(" targeting %q", target)
	}

	details.SideEffects = append(details.SideEffects, effect+".")

	return details
}

// LinodeDomainRecordUpdatePreview reads the record as it stands and reports what
// the update changes about it.
func LinodeDomainRecordUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	recordID := request.GetInt("record_id", 0)
	name := request.GetString("name", "")
	target := request.GetString("target", "")

	return statePreview(ctx, request, cfg, "linode_domain_record_update", method, path, nil,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetDomainRecord(ctx, domainID, recordID)
		},
		func(state any) tools.DryRunDetails {
			return domainRecordUpdateSideEffects(state, name, target)
		})
}

// domainRecordUpdateSideEffects is the Tier B walk for a record update, diffed
// against the fetched state so the preview says what changes rather than what
// was asked for.
func domainRecordUpdateSideEffects(state any, newName, newTarget string) tools.DryRunDetails {
	var (
		details              tools.DryRunDetails
		fromName, fromTarget string
	)

	// A fetch that answered with something else leaves both empty, which reads
	// as "no previous value" rather than as no change at all.
	if record, isRecord := state.(*linode.DomainRecord); isRecord && record != nil {
		fromName, fromTarget = record.Name, record.Target
	}

	if newName != "" && newName != fromName {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Record name changes from %q to %q.", fromName, newName))
	}

	if newTarget != "" && newTarget != fromTarget {
		details.SideEffects = append(details.SideEffects,
			fmt.Sprintf("Record target changes from %q to %q.", fromTarget, newTarget))
	}

	return details
}
