package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ipv6ListPageSizeMin = 25
	ipv6ListPageSizeMax = 500
)

func ipv6ListPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", ipv6ListPageSizeMin, ipv6ListPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}
