#!/bin/bash

# Configuration
API_URL="http://localhost:8080/api/v1/acme-inc/workflows/wf_57135251-a1a4-46/execute"
ITERATIONS=100  # Default number of iterations
DELAY=0.1     # Default delay between requests in seconds
PARALLEL=false # Run requests in parallel or sequential
MAX_PARALLEL=10 # Maximum number of parallel requests

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Request body
REQUEST_BODY='{
    "tenant_id": "acme-inc",
    "input": {
        "user_id": "Bram Purnot",
        "message": "Hello from Multi-Tenant Postman!"
    }
}'

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--iterations)
            ITERATIONS="$2"
            shift 2
            ;;
        -d|--delay)
            DELAY="$2"
            shift 2
            ;;
        -p|--parallel)
            PARALLEL=true
            shift
            ;;
        -m|--max-parallel)
            MAX_PARALLEL="$2"
            shift 2
            ;;
        -u|--url)
            API_URL="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -n, --iterations NUM     Number of times to call the API (default: 50)"
            echo "  -d, --delay SECONDS      Delay between requests in seconds (default: 0.1)"
            echo "  -p, --parallel           Run requests in parallel"
            echo "  -m, --max-parallel NUM   Maximum number of parallel requests (default: 10)"
            echo "  -u, --url URL            Override the API URL"
            echo "  -h, --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                       # Run 50 sequential requests"
            echo "  $0 -n 100               # Run 100 sequential requests"
            echo "  $0 -n 100 -d 0.5        # Run 100 requests with 0.5s delay"
            echo "  $0 -n 100 -p            # Run 100 parallel requests"
            echo "  $0 -n 100 -p -m 20      # Run 100 parallel requests, max 20 at a time"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Function to make a single API call
make_request() {
    local iteration=$1
    local start_time=$(date +%s%N)
    
    # Make the API call
    response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -d "$REQUEST_BODY" 2>/dev/null)
    
    # Extract status code and response body
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | sed '$d')
    
    local end_time=$(date +%s%N)
    local duration=$(( ($end_time - $start_time) / 1000000 )) # Convert to milliseconds
    
    # Output based on status code
    if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
        echo -e "${GREEN}[Request #$iteration]${NC} Status: ${GREEN}$http_code${NC} - Duration: ${duration}ms"
    else
        echo -e "${RED}[Request #$iteration]${NC} Status: ${RED}$http_code${NC} - Duration: ${duration}ms"
        if [[ -n "$response_body" ]]; then
            echo -e "${YELLOW}  Response: ${response_body:0:100}...${NC}"
        fi
    fi
    
    echo "$http_code" >> /tmp/api_test_results_$$.txt
    echo "$duration" >> /tmp/api_test_durations_$$.txt
}

# Function to run requests in parallel
run_parallel() {
    echo -e "${BLUE}Running $ITERATIONS requests in parallel (max $MAX_PARALLEL at a time)...${NC}"
    echo ""
    
    # Use xargs to limit parallel execution
    seq 1 "$ITERATIONS" | xargs -P "$MAX_PARALLEL" -I {} bash -c "$(declare -f make_request); make_request {}"
}

# Function to run requests sequentially
run_sequential() {
    echo -e "${BLUE}Running $ITERATIONS sequential requests with ${DELAY}s delay...${NC}"
    echo ""
    
    for i in $(seq 1 "$ITERATIONS"); do
        make_request "$i"
        
        # Add delay between requests (except after the last one)
        if [[ $i -lt $ITERATIONS ]]; then
            sleep "$DELAY"
        fi
    done
}

# Function to show summary statistics
show_summary() {
    echo ""
    echo -e "${BLUE}=== Test Summary ===${NC}"
    
    if [[ -f /tmp/api_test_results_$$.txt ]]; then
        total=$(wc -l < /tmp/api_test_results_$$.txt)
        success=$(grep -c "^2[0-9][0-9]$" /tmp/api_test_results_$$.txt || echo "0")
        failed=$((total - success))
        
        echo -e "Total Requests: ${YELLOW}$total${NC}"
        echo -e "Successful: ${GREEN}$success${NC}"
        echo -e "Failed: ${RED}$failed${NC}"
        echo -e "Success Rate: ${YELLOW}$(awk "BEGIN {printf \"%.1f%%\", $success/$total*100}")${NC}"
    fi
    
    if [[ -f /tmp/api_test_durations_$$.txt ]]; then
        avg_duration=$(awk '{ sum += $1; n++ } END { if (n > 0) printf "%.0f", sum / n }' /tmp/api_test_durations_$$.txt)
        min_duration=$(sort -n /tmp/api_test_durations_$$.txt | head -1)
        max_duration=$(sort -n /tmp/api_test_durations_$$.txt | tail -1)
        
        echo ""
        echo -e "${BLUE}Response Times:${NC}"
        echo -e "  Average: ${YELLOW}${avg_duration}ms${NC}"
        echo -e "  Min: ${GREEN}${min_duration}ms${NC}"
        echo -e "  Max: ${RED}${max_duration}ms${NC}"
    fi
    
    # Cleanup temporary files
    rm -f /tmp/api_test_results_$$.txt /tmp/api_test_durations_$$.txt
}

# Main execution
echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}   API Load Test Script${NC}"
echo -e "${BLUE}================================${NC}"
echo ""
echo -e "Target URL: ${YELLOW}$API_URL${NC}"
echo -e "Iterations: ${YELLOW}$ITERATIONS${NC}"
echo ""

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    echo -e "${RED}Error: curl is not installed${NC}"
    exit 1
fi

# Start timer
start_time=$(date +%s)

# Run the requests
if [[ "$PARALLEL" == true ]]; then
    run_parallel
else
    run_sequential
fi

# End timer
end_time=$(date +%s)
total_time=$((end_time - start_time))

# Show summary
show_summary
echo ""
echo -e "Total Time: ${YELLOW}${total_time}s${NC}"
echo ""
echo -e "${GREEN}Test completed!${NC}"