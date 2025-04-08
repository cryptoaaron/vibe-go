#!/bin/bash

# Source timestamp functions
source "$(dirname "$0")/timestamp.sh"

# Constants
MDC_FILE=".cursor/rules/paelladoc.mdc"
RULES_DIR=".cursor/rules/generated"

# Create rules directory if it doesn't exist
mkdir -p "$RULES_DIR"

# Function to generate a rule file
generate_rule() {
    local template="$1"
    local output="$2"
    local timestamp=$(get_iso_8601)
    local date=$(get_date_ymd)
    local datetime=$(get_date_ymd_hms)
    
    # Create rule with timestamp
    cat > "$output" << EOF
---
description: "Generated by PAELLADOC on $datetime"
timestamp: "$timestamp"
globs: ["**/*"]
---
{
    "timestamp": "$timestamp",
    "date": "$date",
    "datetime": "$datetime",
    "template": "$template",
    "generated_by": "PAELLADOC"
}
EOF
}

# Main execution
echo "Generating rules with timestamps..."

# Read templates from MDC and generate rules
while IFS= read -r template; do
    rule_name=$(basename "$template" .md)
    rule_file="$RULES_DIR/${rule_name}.rule"
    generate_rule "$template" "$rule_file"
    echo "Generated rule: $rule_file"
done < <(find .cursor/rules/templates -name "*.md")

echo "Rule generation complete." 