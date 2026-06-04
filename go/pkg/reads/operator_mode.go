package reads

func workflowOperatorMode(workflow map[string]any) any {
	return nullableStringValue(stringValue(workflow["operator_mode"]))
}
