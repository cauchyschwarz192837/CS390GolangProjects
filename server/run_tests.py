import json
import subprocess
import os
import sys
import argparse
import re

# ANSI Colors for terminal output
class Colors:
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    RESET = '\033[0m'
    BOLD = '\033[1m'

def run_perf_tests():
    # --- Argument Parsing ---
    parser = argparse.ArgumentParser(description="Run Go performance tests defined in settings.json")
    parser.add_argument("--tags", help="Optional build tags to pass to go run", default=None)
    cli_args = parser.parse_args()
    # ------------------------

    settings_file = 'settings.json'
    
    if not os.path.exists(settings_file):
        print(f"{Colors.RED}Error: {settings_file} not found.{Colors.RESET}")
        sys.exit(1)

    # 1. Load settings
    with open(settings_file, 'r') as f:
        try:
            settings = json.load(f)
        except json.JSONDecodeError as e:
            print(f"{Colors.RED}Error parsing JSON: {e}{Colors.RESET}")
            sys.exit(1)

    test_dir = settings.get('test_dir', '.')
    suite_names = settings.get('test_suite_names', [])
    all_suites = settings.get('test_suites', {})

    # if not os.path.isdir(test_dir):
    #     print(f"{Colors.RED}Error: Test directory '{test_dir}' does not exist.{Colors.RESET}")
    #     sys.exit(1)

    # Tolerance for "reasonable range" (e.g., +/- 20%)
    TOLERANCE = 0.20 

    for suite_name in suite_names:
        print(f"\n{Colors.BOLD}=== Running Performance Suite: {suite_name} ==={Colors.RESET}")
        
        go_file = f"{suite_name}.go"
        if not os.path.exists(go_file):
            print(f"{Colors.RED}Warning: File {go_file} not found. Skipping suite.{Colors.RESET}")
            continue

        tests = all_suites.get(suite_name, [])

        for i, test in enumerate(tests):
            description = test.get('desc', f'Test #{i}')
            args = test.get('args', [])
            points = test.get('points', 0)
            
            # Identify expected parameter indices based on serveload.go usage
            # Usage: go run serveload.go <iatMean> <demandMean> <maxConcurrent>
            if len(args) < 3:
                print(f"{Colors.RED}Error: Test '{description}' missing arguments.{Colors.RESET}")
                continue

            try:
                iat_mean = float(args[0])
                demand_mean = float(args[1])
                max_concurrent = int(args[2])
            except ValueError:
                print(f"{Colors.RED}Error: Invalid arguments for math calculation.{Colors.RESET}")
                continue

            # --- Calculate Expected Performance Metrics ---
            # Units: iatMean and demandMean are in ms. Throughput is req/sec.
            
            # Expected Load (Lambda) = 1000 / iatMean
            expected_lambda = 1000.0 / iat_mean if iat_mean > 0 else 0
            
            # Max Throughput Capacity = (maxConcurrent * 1000) / demandMean
            max_throughput = (max_concurrent * 1000.0) / demand_mean if demand_mean > 0 else 0

            is_saturated = expected_lambda >= max_throughput

            print(f"\nTest {i}: {description}")
            print(f"  Input: IAT={iat_mean}ms, Demand={demand_mean}ms, Concurrent={max_concurrent}")
            print(f"  Calculated: Lambda={expected_lambda:.1f}/sec, MaxCap={max_throughput:.1f}/sec")
            print(f"  Mode: {Colors.YELLOW}{'SATURATED' if is_saturated else 'NOT SATURATED'}{Colors.RESET}")

            # --- Construct Command ---
            cmd = ["go", "run"]
            if cli_args.tags:
                cmd.extend(["-tags", cli_args.tags])
            cmd.append(go_file)
            cmd.extend(args)

            try:
                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    text=True
                )
                
                output = result.stdout
                
                # --- Parse Actual Output ---
                # Looking for: throughput=196/sec meanRT=408.708ms
                tp_match = re.search(r'throughput=([\d\.]+)/sec', output)
                rt_match = re.search(r'meanRT=([\d\.]+)ms', output)

                if not tp_match or not rt_match:
                    print(f"  {Colors.RED}[ERROR] Could not parse output.{Colors.RESET}")
                    print(f"  Stdout: {output.strip()}")
                    continue

                actual_throughput = float(tp_match.group(1))
                actual_mean_rt = float(rt_match.group(1))

                # --- Validation Logic ---
                passed = False
                msg = ""

                if is_saturated:
                    # Metric: Throughput
                    # Condition: Actual throughput should be close to Max Throughput
                    lower_bound = max_throughput * (1.0 - TOLERANCE)
                    upper_bound = max_throughput * (1.0 + TOLERANCE)
                    
                    if lower_bound <= actual_throughput <= upper_bound:
                        passed = True
                        msg = f"Throughput {actual_throughput:.1f} is within range of max {max_throughput:.1f}"
                    else:
                        msg = f"Throughput {actual_throughput:.1f} OUT OF RANGE (Expected ~{max_throughput:.1f})"
                else:
                    # Metric: MeanRT
                    # Condition: Actual MeanRT should be close to DemandMean
                    # Note: RT can only be >= DemandMean. We allow some overhead (tolerance).
                    # A simplistic check: Within 20% or simply 'reasonable' (e.g. up to 1.5x if needed)
                    # Using standard tolerance for now:
                    target_rt = demand_mean
                    lower_bound = target_rt * 0.9 # It physically can't be much lower than service time
                    upper_bound = target_rt * (1.0 + TOLERANCE) 
                    
                    if lower_bound <= actual_mean_rt <= upper_bound:
                        passed = True
                        msg = f"MeanRT {actual_mean_rt:.1f}ms is within range of demand {demand_mean}ms"
                    else:
                        # If unsaturated but RT is high, it might be close to saturation boundary
                        msg = f"MeanRT {actual_mean_rt:.1f}ms OUT OF RANGE (Expected ~{demand_mean}ms)"

                # --- Print Result ---
                if passed:
                    print(f"  {Colors.GREEN}[PASS]{Colors.RESET} {msg}")
                else:
                    print(f"  {Colors.RED}[FAIL]{Colors.RESET} {msg}")
                    print(f"  Actual Output Line: {tp_match.group(0)} {rt_match.group(0)}")

            except Exception as e:
                print(f"  {Colors.RED}[ERROR] Execution failed: {e}{Colors.RESET}")

if __name__ == "__main__":
    run_perf_tests()