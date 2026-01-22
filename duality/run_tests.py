import json
import subprocess
import os
import sys
import difflib
import argparse

# ANSI Colors for terminal output
class Colors:
    GREEN = '\033[92m'
    RED = '\033[91m'
    RESET = '\033[0m'
    BOLD = '\033[1m'

def run_tests():
    # --- NEW: Argument Parsing ---
    parser = argparse.ArgumentParser(description="Run Go tests defined in settings.json")
    parser.add_argument("--tags", help="Optional build tags to pass to go run (e.g., 'lock')", default=None)
    cli_args = parser.parse_args()
    # -----------------------------

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

    # Ensure test directory exists
    if not os.path.isdir(test_dir):
        print(f"{Colors.RED}Error: Test directory '{test_dir}' does not exist.{Colors.RESET}")
        sys.exit(1)

    total_points = 0
    max_points = 0

    # 2. Iterate through suites
    for suite_name in suite_names:
        print(f"\n{Colors.BOLD}=== Running Suite: {suite_name} ==={Colors.RESET}")
        
        go_file = f"{suite_name}.go"
        if not os.path.exists(go_file):
            print(f"{Colors.RED}Warning: File {go_file} not found. Skipping suite.{Colors.RESET}")
            continue

        tests = all_suites.get(suite_name, [])

        for i, test in enumerate(tests):
            description = test.get('desc', f'Test #{i}')
            test_args = test.get('args', [])
            points = test.get('points', 0)
            max_points += points

            print(f"\nTest {i}: {description} (Points: {points})")
            
            # Construct filenames
            expected_path = os.path.join(test_dir, f"{suite_name}_expected_{i}.txt")
            actual_path = os.path.join(test_dir, f"{suite_name}_actual_{i}.txt")
            diff_path = os.path.join(test_dir, f"{suite_name}_diff_{i}.txt")

            # --- NEW: Construct Command with Tags ---
            # Format: go run [-tags <tags>] <file.go> <args>
            cmd = ["go", "run"]
            
            if cli_args.tags:
                print(f"  [Info] Using tags: {cli_args.tags}")
                cmd.extend(["-tags", cli_args.tags])
            
            cmd.append(go_file)
            cmd.extend(test_args)
            # ----------------------------------------

            try:
                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    text=True
                )
                
                actual_output = result.stdout
                
                # Write Actual Output
                with open(actual_path, 'w') as f:
                    f.write(actual_output)

                # Check if Expected file exists
                if not os.path.exists(expected_path):
                    print(f"  {Colors.RED}[ERROR]{Colors.RESET} Expected file not found: {expected_path}")
                    print(f"  Saved actual output to: {actual_path}")
                    continue

                # Read Expected Output
                with open(expected_path, 'r') as f:
                    expected_output = f.read()

                # 4. Compare Outputs
                actual_lines = actual_output.strip().splitlines()
                expected_lines = expected_output.strip().splitlines()

                if actual_lines == expected_lines:
                    # PASS
                    print(f"  {Colors.GREEN}[PASS]{Colors.RESET} Output matches expected.")
                    total_points += points
                    
                    if os.path.exists(diff_path):
                        os.remove(diff_path)
                else:
                    # FAIL
                    print(f"  {Colors.RED}[FAIL]{Colors.RESET} Output mismatch.")
                    
                    diff = difflib.unified_diff(
                        expected_lines, 
                        actual_lines, 
                        fromfile=f'expected_{i}', 
                        tofile=f'actual_{i}',
                        lineterm=''
                    )
                    
                    with open(diff_path, 'w') as f:
                        f.write('\n'.join(diff))
                    
                    print(f"  Saved actual output to: {actual_path}")
                    print(f"  Saved diff to: {diff_path}")

            except Exception as e:
                print(f"  {Colors.RED}[ERROR]{Colors.RESET} Execution failed: {e}")

    # 5. Final Score
    print("\n" + "="*30)
    score_color = Colors.GREEN if total_points == max_points else Colors.RED
    print(f"{Colors.BOLD}Total Score: {score_color}{total_points}/{max_points}{Colors.RESET}")
    print("="*30)

if __name__ == "__main__":
    run_tests()