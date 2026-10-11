#!/usr/bin/env python3
"""
First-pass analysis of esp32-app-foundation transcripts.
Extracts: PA skill usage, bug-fixer patterns, tool calls, token counts,
memory/index reads, trajectory categorization.
"""
import json
import os
import sys
from collections import Counter, defaultdict
from pathlib import Path

TRANSCRIPT_DIR = Path(os.path.expanduser(
    "~/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcripts"
))

def parse_transcript(path):
    """Parse a transcript.jsonl into a list of steps."""
    steps = []
    with open(path, 'r') as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                step = json.loads(line)
                steps.append(step)
            except json.JSONDecodeError:
                pass
    return steps

def analyze_conversation(conv_id, transcript_path):
    """Analyze a single conversation transcript."""
    steps = parse_transcript(transcript_path)
    if not steps:
        return None
    
    result = {
        'conv_id': conv_id,
        'step_count': len(steps),
        'created_at': None,
        'last_at': None,
        # PA skill
        'pa_skill_mentions': 0,
        'pa_workspace_reads': 0,
        'memory_md_reads': 0,
        'project_md_reads': 0,
        'workorder_reads': 0,
        # Bug fixer
        'is_bugfixer': False,
        'bugfixer_mentions': 0,
        # Tool calls
        'tool_calls': Counter(),
        'file_reads': 0,
        'file_writes': 0,
        'commands_run': 0,
        'subagents_spawned': 0,
        'web_searches': 0,
        # Memory system
        'repo_memory_mentions': 0,
        'memory_index_reads': 0,
        'mcp_memory_calls': 0,
        # Git
        'git_commits': 0,
        'git_pr_creates': 0,
        # Content signals
        'has_user_input': False,
        'user_input_count': 0,
        'error_count': 0,
        # Files touched
        'files_read': set(),
        'files_written': set(),
        # Tokens (if available)
        'total_content_bytes': 0,
    }
    
    for step in steps:
        # Timestamps
        ts = step.get('created_at', '')
        if ts:
            if not result['created_at']:
                result['created_at'] = ts
            result['last_at'] = ts
        
        # Source/type
        source = step.get('source', '')
        step_type = step.get('type', '')
        content = step.get('content', '') or ''
        
        if step_type == 'USER_INPUT':
            result['has_user_input'] = True
            result['user_input_count'] += 1
        
        if step.get('status') == 'ERROR':
            result['error_count'] += 1
        
        result['total_content_bytes'] += len(content)
        
        # Check for PA skill references
        if 'principal-architect' in content.lower() or 'principal_architect' in content.lower():
            result['pa_skill_mentions'] += 1
        
        # Check for bug-fixer
        if 'bug-fixer' in content.lower() or 'bugfixer' in content.lower() or 'bug_fixer' in content.lower():
            result['bugfixer_mentions'] += 1
        
        # Check for repo-memory
        if 'repo-memory' in content.lower() or 'repo_memory' in content.lower():
            result['repo_memory_mentions'] += 1
        
        # Tool calls
        tool_calls = step.get('tool_calls', []) or []
        for tc in tool_calls:
            tool_name = tc.get('name', '') or ''
            if not tool_name:
                # Try nested
                tool_name = tc.get('tool_name', '') or ''
            
            if tool_name:
                result['tool_calls'][tool_name] += 1
            
            # Classify tool calls
            args = tc.get('args', {}) or tc.get('arguments', {}) or {}
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except:
                    args = {}
            
            if tool_name in ('view_file', 'read_file'):
                result['file_reads'] += 1
                path = args.get('AbsolutePath', '') or args.get('path', '') or ''
                if path:
                    result['files_read'].add(path)
                    if 'principal-architect-workspace' in path:
                        result['pa_workspace_reads'] += 1
                        if 'MEMORY.md' in path:
                            result['memory_md_reads'] += 1
                        if 'PROJECT.md' in path:
                            result['project_md_reads'] += 1
                        if 'WORKORDER.md' in path:
                            result['workorder_reads'] += 1
            
            elif tool_name in ('write_to_file', 'replace_file_content', 'create_file'):
                result['file_writes'] += 1
                path = args.get('TargetFile', '') or args.get('path', '') or ''
                if path:
                    result['files_written'].add(path)
            
            elif tool_name == 'run_command':
                result['commands_run'] += 1
                cmd = args.get('CommandLine', '') or args.get('command', '') or ''
                if 'git commit' in cmd:
                    result['git_commits'] += 1
                if 'gh pr create' in cmd or 'git push' in cmd:
                    result['git_pr_creates'] += 1
            
            elif tool_name == 'invoke_subagent':
                result['subagents_spawned'] += 1
            
            elif tool_name == 'search_web':
                result['web_searches'] += 1
            
            elif 'memory' in tool_name.lower() and ('mcp' in tool_name.lower() or 'call_mcp' in tool_name):
                result['mcp_memory_calls'] += 1
            
            elif tool_name == 'call_mcp_tool':
                mcp_tool = args.get('ToolName', '') or ''
                if 'memory' in mcp_tool.lower():
                    result['mcp_memory_calls'] += 1
        
        # Check content for bug-fixer daemon markers
        if 'bug-fixer-daemon' in content or 'BugFixerDaemon' in content:
            result['is_bugfixer'] = True
    
    # Determine trajectory type
    if result['is_bugfixer'] or result['bugfixer_mentions'] > 3:
        result['trajectory_type'] = 'bugfixer'
    elif result['pa_skill_mentions'] > 2 or result['pa_workspace_reads'] > 0:
        result['trajectory_type'] = 'pa_driven'
    elif result['subagents_spawned'] > 0:
        result['trajectory_type'] = 'multi_agent'
    elif result['has_user_input']:
        result['trajectory_type'] = 'interactive'
    else:
        result['trajectory_type'] = 'autonomous'
    
    # Convert sets to counts for JSON serialization
    result['unique_files_read'] = len(result['files_read'])
    result['unique_files_written'] = len(result['files_written'])
    result['tool_calls'] = dict(result['tool_calls'])
    del result['files_read']
    del result['files_written']
    
    return result

def main():
    if not TRANSCRIPT_DIR.exists():
        print(f"Directory not found: {TRANSCRIPT_DIR}")
        sys.exit(1)
    
    results = []
    conv_dirs = sorted(TRANSCRIPT_DIR.iterdir())
    
    print(f"Analyzing {len(conv_dirs)} conversations...", file=sys.stderr)
    
    for conv_dir in conv_dirs:
        if not conv_dir.is_dir():
            continue
        transcript = conv_dir / "transcript.jsonl"
        if not transcript.exists():
            continue
        
        conv_id = conv_dir.name
        result = analyze_conversation(conv_id, transcript)
        if result:
            results.append(result)
    
    # Summary statistics
    print(f"\n{'='*70}")
    print(f"ESP32-APP-FOUNDATION TRANSCRIPT ANALYSIS")
    print(f"{'='*70}")
    print(f"\nTotal conversations: {len(results)}")
    
    # Date range
    dates = [r['created_at'] for r in results if r['created_at']]
    if dates:
        print(f"Date range: {min(dates)[:10]} to {max(dates)[:10]}")
    
    # Trajectory types
    print(f"\n--- TRAJECTORY TYPES ---")
    type_counts = Counter(r['trajectory_type'] for r in results)
    for t, c in type_counts.most_common():
        print(f"  {t:20s} {c:4d} ({100*c/len(results):.1f}%)")
    
    # PA skill usage
    print(f"\n--- PA SKILL USAGE ---")
    pa_users = [r for r in results if r['pa_workspace_reads'] > 0]
    print(f"  Conversations reading PA workspace: {len(pa_users)} ({100*len(pa_users)/len(results):.1f}%)")
    pa_mentions = [r for r in results if r['pa_skill_mentions'] > 0]
    print(f"  Conversations mentioning PA skill:  {len(pa_mentions)} ({100*len(pa_mentions)/len(results):.1f}%)")
    print(f"  Total PA workspace reads:           {sum(r['pa_workspace_reads'] for r in results)}")
    print(f"  Total MEMORY.md reads:              {sum(r['memory_md_reads'] for r in results)}")
    print(f"  Total PROJECT.md reads:             {sum(r['project_md_reads'] for r in results)}")
    print(f"  Total WORKORDER.md reads:           {sum(r['workorder_reads'] for r in results)}")
    
    # Bug-fixer
    print(f"\n--- BUG-FIXER ---")
    bf = [r for r in results if r['trajectory_type'] == 'bugfixer']
    print(f"  Bug-fixer conversations: {len(bf)} ({100*len(bf)/len(results):.1f}%)")
    
    # Memory system
    print(f"\n--- MEMORY SYSTEM ---")
    mem_mentions = [r for r in results if r['repo_memory_mentions'] > 0]
    print(f"  Conversations mentioning repo-memory: {len(mem_mentions)}")
    mcp_users = [r for r in results if r['mcp_memory_calls'] > 0]
    print(f"  Conversations using MCP memory tools: {len(mcp_users)}")
    print(f"  Total MCP memory tool calls:          {sum(r['mcp_memory_calls'] for r in results)}")
    
    # Tool call summary
    print(f"\n--- TOOL USAGE (across all conversations) ---")
    all_tools = Counter()
    for r in results:
        for tool, count in r['tool_calls'].items():
            all_tools[tool] += count
    for tool, count in all_tools.most_common(20):
        print(f"  {tool:40s} {count:6d}")
    
    # Scale stats
    print(f"\n--- SCALE ---")
    steps = [r['step_count'] for r in results]
    print(f"  Total steps:    {sum(steps):,}")
    print(f"  Mean steps:     {sum(steps)/len(steps):.1f}")
    print(f"  Median steps:   {sorted(steps)[len(steps)//2]}")
    print(f"  Max steps:      {max(steps)}")
    
    file_reads = [r['file_reads'] for r in results]
    print(f"  Total file reads:  {sum(file_reads):,}")
    file_writes = [r['file_writes'] for r in results]
    print(f"  Total file writes: {sum(file_writes):,}")
    commands = [r['commands_run'] for r in results]
    print(f"  Total commands:    {sum(commands):,}")
    subagents = [r['subagents_spawned'] for r in results]
    print(f"  Total subagents:   {sum(subagents):,}")
    commits = [r['git_commits'] for r in results]
    print(f"  Total git commits: {sum(commits):,}")
    
    content_mb = sum(r['total_content_bytes'] for r in results) / 1024 / 1024
    print(f"  Total content:     {content_mb:.1f} MB")
    
    # Per trajectory type breakdown
    print(f"\n--- PER-TYPE BREAKDOWN ---")
    for ttype in ['pa_driven', 'bugfixer', 'interactive', 'multi_agent', 'autonomous']:
        subset = [r for r in results if r['trajectory_type'] == ttype]
        if not subset:
            continue
        avg_steps = sum(r['step_count'] for r in subset) / len(subset)
        avg_reads = sum(r['file_reads'] for r in subset) / len(subset)
        avg_writes = sum(r['file_writes'] for r in subset) / len(subset)
        avg_cmds = sum(r['commands_run'] for r in subset) / len(subset)
        avg_subs = sum(r['subagents_spawned'] for r in subset) / len(subset)
        avg_bytes = sum(r['total_content_bytes'] for r in subset) / len(subset) / 1024
        print(f"\n  {ttype} (n={len(subset)}):")
        print(f"    Avg steps:       {avg_steps:.0f}")
        print(f"    Avg file reads:  {avg_reads:.0f}")
        print(f"    Avg file writes: {avg_writes:.0f}")
        print(f"    Avg commands:    {avg_cmds:.0f}")
        print(f"    Avg subagents:   {avg_subs:.1f}")
        print(f"    Avg content:     {avg_bytes:.0f} KB")
    
    # User input patterns
    print(f"\n--- USER INTERACTION ---")
    interactive = [r for r in results if r['has_user_input']]
    print(f"  Conversations with user input: {len(interactive)} ({100*len(interactive)/len(results):.1f}%)")
    if interactive:
        avg_inputs = sum(r['user_input_count'] for r in interactive) / len(interactive)
        print(f"  Avg user inputs per interactive conv: {avg_inputs:.1f}")
    
    # Errors
    error_convs = [r for r in results if r['error_count'] > 0]
    print(f"  Conversations with errors: {len(error_convs)} ({100*len(error_convs)/len(results):.1f}%)")
    
    # Output per-conversation data as JSONL for further analysis
    output_path = TRANSCRIPT_DIR.parent / "esp32-transcript-analysis.jsonl"
    with open(output_path, 'w') as f:
        for r in results:
            json.dump(r, f)
            f.write('\n')
    print(f"\nDetailed per-conversation data: {output_path}")

if __name__ == '__main__':
    main()
