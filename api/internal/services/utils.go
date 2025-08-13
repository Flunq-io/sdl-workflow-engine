package services

import (
	"strings"
	"time"
)

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// parseCommaSeparated splits a comma-separated string into a slice
func parseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// hasAnyTag checks if any of the filterTags exist in workflowTags
func hasAnyTag(workflowTags, filterTags []string) bool {
	for _, filterTag := range filterTags {
		for _, workflowTag := range workflowTags {
			if strings.EqualFold(workflowTag, filterTag) {
				return true
			}
		}
	}
	return false
}

// isInDateRange checks if a date is within the specified range
func isInDateRange(date time.Time, dateRange string) bool {
	if dateRange == "" {
		return true
	}
	
	parts := strings.Split(dateRange, ",")
	if len(parts) != 2 {
		return true // Invalid range format, include all
	}
	
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])
	
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return true // Invalid start date, include all
	}
	
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return true // Invalid end date, include all
	}
	
	// Add 24 hours to end date to include the entire day
	end = end.Add(24 * time.Hour)
	
	return date.After(start) && date.Before(end)
}
