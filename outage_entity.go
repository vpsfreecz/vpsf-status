package main

import "strings"

func (e OutageEntity) EffectiveType() string {
	if e.EntityType != "" {
		return normalizeOutageEntityType(e.EntityType)
	}

	return normalizeOutageEntityType(e.Name)
}

func (e OutageEntity) DisplayLabel() string {
	label := strings.TrimSpace(e.Label)
	if label == "" {
		return strings.TrimSpace(e.Name)
	}

	prefix := legacyOutageEntityLabelPrefix(e.EffectiveType())
	if prefix != "" && strings.HasPrefix(label, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(label, prefix))
	}

	return label
}

func normalizeOutageEntityType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "vpsadmin", "vps_admin":
		return "vpsadmin"
	case "cluster":
		return "cluster"
	case "environment":
		return "environment"
	case "location":
		return "location"
	case "node":
		return "node"
	default:
		return "custom"
	}
}

func legacyOutageEntityLabelPrefix(entityType string) string {
	switch entityType {
	case "vpsadmin":
		return "vpsAdmin "
	case "cluster":
		return "Cluster "
	case "environment":
		return "Environment "
	case "location":
		return "Location "
	case "node":
		return "Node "
	default:
		return ""
	}
}
