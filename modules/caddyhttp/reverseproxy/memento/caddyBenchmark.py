#!/usr/bin/env python3
"""
Benchmark Visualization Script
Reads benchmark_results.csv and generates comparison charts
"""

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
import argparse
import os


def load_data(csv_file='benchmark_results.csv'):
    """Load benchmark data from CSV and convert to expected format"""
    if not os.path.exists(csv_file):
        print(f"Error: {csv_file} not found!")
        return None

    df = pd.read_csv(csv_file)
    print(f"Loaded {len(df)} benchmark results from {csv_file}")
    
    # Convert to expected format if needed
    if 'Scenario' not in df.columns:
        # Convert from new format to expected format
        df = convert_benchmark_format(df)
    
    return df


def convert_benchmark_format(df):
    """Convert benchmark CSV format to expected format with Scenario, Algorithm, etc."""
    # Create new dataframe with expected columns
    converted_data = []
    
    for _, row in df.iterrows():
        benchmark = row['Benchmark']
        ns_per_op = row['NsPerOp']
        alloc_bytes = row['AllocBytes']
        allocs_per_op = row['AllocsPerOp']
        
        # Parse benchmark name to extract scenario and algorithm
        scenario, algorithm, test_name = parse_benchmark_name(benchmark)
        
        converted_data.append({
            'Scenario': scenario,
            'Algorithm': algorithm,
            'TestName': test_name,
            'TimeNs': ns_per_op,
            'AllocBytes': alloc_bytes,
            'AllocsPerOp': allocs_per_op,
            'CPU': row.get('CPU', ''),
            'GOOS': row.get('GOOS', ''),
            'GOARCH': row.get('GOARCH', '')
        })
    
    return pd.DataFrame(converted_data)


def parse_benchmark_name(benchmark):
    """Parse benchmark name to extract scenario, algorithm, and test name"""
    # Default values
    scenario = 'Other'
    algorithm = 'Memento'
    test_name = benchmark
    
    # Map benchmark names to scenarios
    if 'BinomialEngineGetBucket' in benchmark:
        if 'DifferentKeys' in benchmark:
            scenario = 'Different Keys'
            algorithm = 'BinomialEngine'
        else:
            scenario = 'Same Key'
            algorithm = 'BinomialEngine'
    elif 'RemoveBucket' in benchmark:
        if 'Sequential' in benchmark:
            scenario = 'Node Removal'
            algorithm = 'Memento'
            test_name = 'RemoveBucket_Sequential'
        elif 'WithLookups' in benchmark:
            scenario = 'Node Removal with Lookups'
            algorithm = 'Memento'
            test_name = 'RemoveBucket_WithLookups'
        else:
            scenario = 'Node Removal'
            algorithm = 'Memento'
    elif '_100Nodes_' in benchmark and ('Removed' in benchmark or 'Unavailable' in benchmark):
        # Progressive removals: Memento_100Nodes_XRemoved or Rendezvous_100Nodes_XUnavailable
        if 'Memento' in benchmark and 'Removed' in benchmark:
            scenario = 'Progressive Removals'
            algorithm = 'Memento'
            # Extract number: Memento_100Nodes_50Removed -> 50
            try:
                removed_str = benchmark.split('Removed')[0].split('_')[-1]
                test_name = f'Memento_100Nodes_{removed_str}Removed'
            except:
                test_name = benchmark
        elif 'Rendezvous' in benchmark and 'Unavailable' in benchmark:
            scenario = 'Progressive Removals'
            algorithm = 'Rendezvous'
            # Extract number: Rendezvous_100Nodes_50Unavailable -> 50
            try:
                removed_str = benchmark.split('Unavailable')[0].split('_')[-1]
                test_name = f'Rendezvous_100Nodes_{removed_str}Unavailable'
            except:
                test_name = benchmark
        else:
            scenario = 'Progressive Removals'
            algorithm = 'Memento'
            test_name = benchmark
    elif 'RemoveNode' in benchmark:
        if 'ConsistentEngine' in benchmark:
            scenario = 'Consistent Engine Node Removal'
            algorithm = 'Memento'
            test_name = 'RemoveNode_ConsistentEngine'
        elif 'WithLookups' in benchmark:
            scenario = 'Consistent Engine Removal with Lookups'
            algorithm = 'Memento'
            test_name = 'RemoveNode_WithLookups'
        else:
            scenario = 'Consistent Engine Node Removal'
            algorithm = 'Memento'
    elif 'MementoSizeAccess' in benchmark:
        scenario = 'Size Access'
        algorithm = 'Memento'
        test_name = 'SizeAccess'
    elif 'MementoRemember' in benchmark:
        scenario = 'Remember Operation'
        algorithm = 'Memento'
        test_name = 'Remember'
    elif 'MementoReplacer' in benchmark:
        scenario = 'Replacer Lookup'
        algorithm = 'Memento'
        test_name = 'Replacer'
    elif 'ConsistentEngineGetBucket' in benchmark:
        scenario = 'Consistent Engine GetBucket'
        algorithm = 'Memento'
        test_name = 'ConsistentEngineGetBucket'
    else:
        scenario = 'Other'
        algorithm = 'Memento'
        test_name = benchmark
    
    return scenario, algorithm, test_name


def plot_same_key_comparison(df):
    """Plot same key performance comparison"""
    same_key_data = df[df['Scenario'] == 'Same Key']

    if len(same_key_data) == 0:
        print("No same key data found")
        return

    fig, ax1 = plt.subplots(1, 1, figsize=(12, 6))

    # Get available algorithms (BinomialEngine, Memento, etc.)
    available_algs = same_key_data['Algorithm'].unique()
    
    # Calculate average times for each algorithm
    alg_times = {}
    for alg in available_algs:
        alg_data = same_key_data[same_key_data['Algorithm'] == alg]['TimeNs']
        if len(alg_data) > 0:
            alg_times[alg] = alg_data.mean()

    if not alg_times:
        print("No valid time data found")
        return

    ordered_algs = list(alg_times.keys())
    times = [alg_times[alg] for alg in ordered_algs]
    
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF', 'BinomialEngine': '#4ECDC4'}
    colors = [color_map.get(alg, '#999999') for alg in ordered_algs]

    bars = ax1.bar(ordered_algs, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Same Key Performance Comparison')
    ax1.grid(True, alpha=0.3)

    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                 f'{time:.1f} ns', ha='center', va='bottom', fontweight='bold')

    plt.tight_layout()
    plt.savefig('same_key_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_different_keys_comparison(df):
    """Plot different keys performance comparison"""
    diff_keys_data = df[df['Scenario'] == 'Different Keys']

    if len(diff_keys_data) == 0:
        print("No different keys data found")
        return

    fig, ax1 = plt.subplots(1, 1, figsize=(12, 6))

    # Get available algorithms
    available_algs = diff_keys_data['Algorithm'].unique()
    
    # Calculate average times for each algorithm
    alg_times = {}
    for alg in available_algs:
        alg_data = diff_keys_data[diff_keys_data['Algorithm'] == alg]['TimeNs']
        if len(alg_data) > 0:
            alg_times[alg] = alg_data.mean()

    if not alg_times:
        print("No valid time data found")
        return

    ordered_algs = list(alg_times.keys())
    times = [alg_times[alg] for alg in ordered_algs]
    
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF', 'BinomialEngine': '#4ECDC4'}
    colors = [color_map.get(alg, '#999999') for alg in ordered_algs]

    bars = ax1.bar(ordered_algs, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Different Keys Performance Comparison')
    ax1.grid(True, alpha=0.3)

    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                 f'{time:.0f} ns', ha='center', va='bottom', fontweight='bold')

    plt.tight_layout()
    plt.savefig('different_keys_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_uri_hash_comparison(df):
    """Plot URI hash performance comparison"""
    uri_data = df[df['Scenario'].str.contains('URI', na=False)]

    if len(uri_data) == 0:
        print("No URI hash data found")
        return

    fig, ax = plt.subplots(figsize=(12, 8))

    # Group by scenario and algorithm
    same_uri = uri_data[uri_data['Scenario'] == 'Same URI']
    diff_uri = uri_data[uri_data['Scenario'] == 'Different URIs']

    algorithms = uri_data['Algorithm'].unique()
    x = np.arange(len(algorithms))
    width = 0.35

    same_times = []
    diff_times = []
    
    for alg in algorithms:
        same_alg = same_uri[same_uri['Algorithm'] == alg]
        diff_alg = diff_uri[diff_uri['Algorithm'] == alg]
        
        same_time = same_alg['TimeNs'].mean() if len(same_alg) > 0 else 0
        diff_time = diff_alg['TimeNs'].mean() if len(diff_alg) > 0 else 0
        
        same_times.append(same_time)
        diff_times.append(diff_time)

    bars1 = ax.bar(x - width / 2, same_times, width, label='Same URI',
                   color='#FF6B6B', alpha=0.7)
    bars2 = ax.bar(x + width / 2, diff_times, width, label='Different URIs',
                   color='#4ECDC4', alpha=0.7)

    ax.set_xlabel('Algorithm')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('URI Hash Performance Comparison')
    ax.set_xticks(x)
    ax.set_xticklabels(algorithms)
    ax.legend()
    ax.grid(True, alpha=0.3)

    # Add value labels
    for bars in [bars1, bars2]:
        for bar in bars:
            height = bar.get_height()
            if height > 0:
                ax.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                        f'{height:.0f}', ha='center', va='bottom', fontweight='bold')

    plt.tight_layout()
    plt.savefig('uri_hash_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_pool_size_scalability(df):
    """Plot scalability with different pool sizes"""
    pool_data = df[df['Scenario'] == 'Pool Size Scalability']

    if len(pool_data) == 0:
        print("No pool size scalability data found")
        return

    fig, ax = plt.subplots(figsize=(12, 8))

    # Separate data by algorithm
    memento_data = pool_data[pool_data['Algorithm'] == 'Memento']
    rendezvous_data = pool_data[pool_data['Algorithm'] == 'Rendezvous']

    # Extract pool sizes and times for Memento
    memento_sizes = []
    memento_times = []

    for _, row in memento_data.iterrows():
        test_name = row['TestName']
        if 'PoolSize_' in test_name:
            size_str = test_name.split('PoolSize_')[1]
            try:
                size = int(size_str)
                memento_sizes.append(size)
                memento_times.append(row['TimeNs'])
            except ValueError:
                continue

    # Extract pool sizes and times for Rendezvous
    rendezvous_sizes = []
    rendezvous_times = []

    for _, row in rendezvous_data.iterrows():
        test_name = row['TestName']
        if 'PoolSize_' in test_name:
            size_str = test_name.split('PoolSize_')[1]
            try:
                size = int(size_str)
                rendezvous_sizes.append(size)
                rendezvous_times.append(row['TimeNs'])
            except ValueError:
                continue

    # Sort by pool size
    if memento_sizes:
        memento_sorted = sorted(zip(memento_sizes, memento_times))
        memento_sizes, memento_times = zip(*memento_sorted)

    if rendezvous_sizes:
        rendezvous_sorted = sorted(zip(rendezvous_sizes, rendezvous_times))
        rendezvous_sizes, rendezvous_times = zip(*rendezvous_sorted)

    # Plot algorithms
    if memento_sizes:
        ax.plot(memento_sizes, memento_times, marker='o', linewidth=3, markersize=8,
                color='#1E90FF', label='Memento')

        # Add value labels for Memento
        for x, y in zip(memento_sizes, memento_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points",
                        xytext=(0, 10), ha='center', fontweight='bold', fontsize=8)

    if rendezvous_sizes:
        ax.plot(rendezvous_sizes, rendezvous_times, marker='s', linewidth=3, markersize=8,
                color='#FF6B6B', label='Rendezvous')

        # Add value labels for Rendezvous
        for x, y in zip(rendezvous_sizes, rendezvous_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points",
                        xytext=(0, -15), ha='center', fontweight='bold', fontsize=8)

    ax.set_xlabel('Pool Size (Number of Upstreams)')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Pool Size Scalability: Rendezvous vs Memento')
    ax.grid(True, alpha=0.3)
    ax.legend()

    plt.tight_layout()
    plt.savefig('pool_size_scalability.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_concurrent_access(df):
    """Plot concurrent access performance"""
    concurrent_data = df[df['Scenario'] == 'Concurrent Access']

    if len(concurrent_data) == 0:
        print("No concurrent access data found")
        return

    fig, ax1 = plt.subplots(1, 1, figsize=(12, 6))

    # Calculate average times for each algorithm
    available_algs = concurrent_data['Algorithm'].unique()
    alg_times = {}
    for alg in available_algs:
        alg_data = concurrent_data[concurrent_data['Algorithm'] == alg]['TimeNs']
        if len(alg_data) > 0:
            alg_times[alg] = alg_data.mean()

    if not alg_times:
        print("No valid time data found")
        return

    algorithms = list(alg_times.keys())
    times = [alg_times[alg] for alg in algorithms]
    
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF', 'BinomialEngine': '#4ECDC4'}
    colors = [color_map.get(alg, '#999999') for alg in algorithms]

    bars = ax1.bar(algorithms, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Concurrent Access Performance')
    ax1.grid(True, alpha=0.3)

    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                 f'{time:.1f} ns', ha='center', va='bottom', fontweight='bold')

    plt.tight_layout()
    plt.savefig('concurrent_access_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_comprehensive_comparison(df):
    """Plot comprehensive comparison across all scenarios"""
    fig, ax = plt.subplots(figsize=(16, 10))

    # Group data by scenario
    scenarios = df['Scenario'].unique()
    scenarios = [s for s in scenarios if s not in ['Pool Size Scalability', 'Other']]  # Exclude scalability and other

    if len(scenarios) == 0:
        print("No valid scenarios found")
        return

    # Get all available algorithms
    all_algorithms = df['Algorithm'].unique()
    
    x = np.arange(len(scenarios))
    width = 0.8 / len(all_algorithms)  # Adjust width based on number of algorithms

    # Plot each algorithm
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF', 'BinomialEngine': '#4ECDC4'}
    offset = -width * (len(all_algorithms) - 1) / 2
    
    for alg in all_algorithms:
        alg_times = []
        for scenario in scenarios:
            scenario_data = df[(df['Scenario'] == scenario) & (df['Algorithm'] == alg)]
            if len(scenario_data) > 0:
                avg_time = scenario_data['TimeNs'].mean()
                alg_times.append(avg_time)
            else:
                alg_times.append(0)
        
        bars = ax.bar(x + offset, alg_times, width, label=alg,
                     color=color_map.get(alg, '#999999'), alpha=0.7)
        offset += width
        
        # Add value labels
        for bar, time in zip(bars, alg_times):
            if time > 0:
                height = bar.get_height()
                ax.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                        f'{time:.1f}', ha='center', va='bottom', fontweight='bold', fontsize=7)

    ax.set_xlabel('Scenario')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Comprehensive Performance Comparison')
    ax.set_xticks(x)
    ax.set_xticklabels(scenarios, rotation=45, ha='right')
    ax.legend()
    ax.grid(True, alpha=0.3)

    plt.tight_layout()
    plt.savefig('comprehensive_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_fixed_removals(df):
    """Plot performance with fixed number of removed nodes"""
    removals_data = df[df['Scenario'] == 'Fixed Removals']

    if len(removals_data) == 0:
        print("No fixed removals data found")
        return

    fig, ax = plt.subplots(figsize=(12, 8))

    # Separate data by algorithm
    memento_data = removals_data[removals_data['Algorithm'] == 'Memento']
    rendezvous_data = removals_data[removals_data['Algorithm'] == 'Rendezvous']

    # Extract pool sizes from test names
    memento_sizes = []
    memento_times = []

    for _, row in memento_data.iterrows():
        test_name = row['TestName']
        if 'Nodes_' in test_name:
            # Extract size from format "Memento_20Nodes_5Removed"
            size_str = test_name.split('Nodes_')[0].replace('Memento_', '')
            try:
                size = int(size_str)
                memento_sizes.append(size)
                memento_times.append(row['TimeNs'])
            except ValueError:
                continue

    rendezvous_sizes = []
    rendezvous_times = []

    for _, row in rendezvous_data.iterrows():
        test_name = row['TestName']
        if 'Nodes_' in test_name:
            size_str = test_name.split('Nodes_')[0].replace('Rendezvous_', '')
            try:
                size = int(size_str)
                rendezvous_sizes.append(size)
                rendezvous_times.append(row['TimeNs'])
            except ValueError:
                continue

    # Sort by pool size
    if memento_sizes:
        memento_sorted = sorted(zip(memento_sizes, memento_times))
        memento_sizes, memento_times = zip(*memento_sorted)

    if rendezvous_sizes:
        rendezvous_sorted = sorted(zip(rendezvous_sizes, rendezvous_times))
        rendezvous_sizes, rendezvous_times = zip(*rendezvous_sorted)

    # Plot algorithms
    if memento_sizes:
        ax.plot(memento_sizes, memento_times, marker='o', linewidth=3, markersize=8,
                color='#1E90FF', label='Memento (5 removed)')

        for x, y in zip(memento_sizes, memento_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points",
                        xytext=(0, 10), ha='center', fontweight='bold', fontsize=8)

    if rendezvous_sizes:
        ax.plot(rendezvous_sizes, rendezvous_times, marker='s', linewidth=3, markersize=8,
                color='#FF6B6B', label='Rendezvous (5 unavailable)')

        for x, y in zip(rendezvous_sizes, rendezvous_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points",
                        xytext=(0, -15), ha='center', fontweight='bold', fontsize=8)

    ax.set_xlabel('Total Pool Size')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Performance with 5 Nodes Removed/Unavailable')
    ax.grid(True, alpha=0.3)
    ax.legend()

    plt.tight_layout()
    plt.savefig('fixed_removals.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_progressive_removals(df):
    """Plot performance with progressive node removals - shows lines starting from 0 removed nodes"""
    removals_data = df[df['Scenario'] == 'Progressive Removals']

    if len(removals_data) == 0:
        print("No progressive removals data found")
        print("Note: This chart requires benchmark data with format: Memento_100Nodes_XRemoved or Rendezvous_100Nodes_XUnavailable")
        return

    fig, ax = plt.subplots(figsize=(14, 8))

    # Separate data by algorithm
    memento_data = removals_data[removals_data['Algorithm'] == 'Memento']
    rendezvous_data = removals_data[removals_data['Algorithm'] == 'Rendezvous']

    # Extract number of removed nodes and calculate average times
    memento_dict = {}  # {num_removed: [times]}
    rendezvous_dict = {}  # {num_removed: [times]}

    for _, row in memento_data.iterrows():
        test_name = row['TestName']
        if 'Removed' in test_name:
            # Extract number from format "Memento_100Nodes_50Removed"
            try:
                removed_str = test_name.split('Removed')[0].split('_')[-1]
                removed = int(removed_str)
                if removed not in memento_dict:
                    memento_dict[removed] = []
                memento_dict[removed].append(row['TimeNs'])
            except (ValueError, IndexError):
                continue

    for _, row in rendezvous_data.iterrows():
        test_name = row['TestName']
        if 'Unavailable' in test_name:
            try:
                removed_str = test_name.split('Unavailable')[0].split('_')[-1]
                removed = int(removed_str)
                if removed not in rendezvous_dict:
                    rendezvous_dict[removed] = []
                rendezvous_dict[removed].append(row['TimeNs'])
            except (ValueError, IndexError):
                continue

    # Calculate averages and sort
    memento_removed = []
    memento_times = []
    for removed in sorted(memento_dict.keys()):
        memento_removed.append(removed)
        memento_times.append(np.mean(memento_dict[removed]))

    rendezvous_removed = []
    rendezvous_times = []
    for removed in sorted(rendezvous_dict.keys()):
        rendezvous_removed.append(removed)
        rendezvous_times.append(np.mean(rendezvous_dict[removed]))

    # Plot algorithms
    if memento_removed:
        ax.plot(memento_removed, memento_times, marker='o', linewidth=3, markersize=10,
                color='#1E90FF', label='Memento', linestyle='-')

        for x, y in zip(memento_removed, memento_times):
            ax.annotate(f'{y:.1f}', (x, y), textcoords="offset points",
                        xytext=(0, 12), ha='center', fontweight='bold', fontsize=9, color='#1E90FF')

    if rendezvous_removed:
        ax.plot(rendezvous_removed, rendezvous_times, marker='s', linewidth=3, markersize=10,
                color='#FF6B6B', label='Rendezvous', linestyle='--')

        for x, y in zip(rendezvous_removed, rendezvous_times):
            ax.annotate(f'{y:.1f}', (x, y), textcoords="offset points",
                        xytext=(0, -18), ha='center', fontweight='bold', fontsize=9, color='#FF6B6B')

    # If we have data starting from 0, show the baseline
    if memento_removed and memento_removed[0] == 0:
        ax.axhline(y=memento_times[0], color='#1E90FF', linestyle=':', alpha=0.5, linewidth=1, 
                  label='Memento Baseline (0 removed)')
    if rendezvous_removed and rendezvous_removed[0] == 0:
        ax.axhline(y=rendezvous_times[0], color='#FF6B6B', linestyle=':', alpha=0.5, linewidth=1,
                  label='Rendezvous Baseline (0 removed)')

    ax.set_xlabel('Number of Nodes Removed', fontsize=12, fontweight='bold')
    ax.set_ylabel('Time (ns/op)', fontsize=12, fontweight='bold')
    ax.set_title('Performance with Progressive Node Removals\n(100 total nodes, starting from all nodes active)', 
                fontsize=14, fontweight='bold')
    ax.grid(True, alpha=0.3, linestyle='--')
    ax.legend(loc='best', fontsize=11)
    
    # Set x-axis to show all removed nodes clearly
    if memento_removed or rendezvous_removed:
        all_removed = sorted(set(list(memento_removed) + list(rendezvous_removed)))
        ax.set_xticks(all_removed)
        ax.set_xlim(-1, max(all_removed) + 1 if all_removed else 50)

    plt.tight_layout()
    plt.savefig('progressive_removals.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_node_removal_comparison(df):
    """Plot node removal performance comparison"""
    # Get all removal-related scenarios
    removal_scenarios = ['Node Removal', 'Node Removal with Lookups', 
                        'Consistent Engine Node Removal', 'Consistent Engine Removal with Lookups']
    
    removal_data = df[df['Scenario'].isin(removal_scenarios)]
    
    if len(removal_data) == 0:
        print("No node removal data found")
        return
    
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(16, 6))
    
    # Group by scenario and calculate averages
    scenario_times = {}
    for scenario in removal_scenarios:
        scenario_df = removal_data[removal_data['Scenario'] == scenario]
        if len(scenario_df) > 0:
            scenario_times[scenario] = scenario_df['TimeNs'].mean()
    
    if not scenario_times:
        print("No valid removal data found")
        return
    
    # Bar chart for different removal scenarios
    scenarios = list(scenario_times.keys())
    times = [scenario_times[s] for s in scenarios]
    
    # Shorten scenario names for display
    display_names = [s.replace('Consistent Engine ', '').replace('Node ', '') for s in scenarios]
    
    bars = ax1.bar(display_names, times, color='#1E90FF', alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Node Removal Performance - Memento')
    ax1.grid(True, alpha=0.3)
    ax1.set_xticklabels(display_names, rotation=45, ha='right')
    
    # Add value labels
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                 f'{time:.4f} ns', ha='center', va='bottom', fontweight='bold', fontsize=9)
    
    # Comparison: Sequential vs With Lookups
    sequential_time = scenario_times.get('Node Removal', 0)
    with_lookups_time = scenario_times.get('Node Removal with Lookups', 0)
    
    if sequential_time > 0 and with_lookups_time > 0:
        comparison_data = {
            'Sequential\nRemoval': sequential_time,
            'Removal with\nLookups': with_lookups_time
        }
        
        bars2 = ax2.bar(list(comparison_data.keys()), list(comparison_data.values()),
                       color=['#4ECDC4', '#FF6B6B'], alpha=0.7, edgecolor='black')
        ax2.set_ylabel('Time (ns/op)')
        ax2.set_title('Removal: Sequential vs With Lookups')
        ax2.grid(True, alpha=0.3)
        
        for bar, time in zip(bars2, list(comparison_data.values())):
            height = bar.get_height()
            ax2.text(bar.get_x() + bar.get_width() / 2., height + height * 0.01,
                     f'{time:.4f} ns', ha='center', va='bottom', fontweight='bold', fontsize=9)
    
    plt.tight_layout()
    plt.savefig('node_removal_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()


def plot_all_charts(df):
    """Generate all comparison charts"""
    print("Generating all benchmark comparison charts...")

    # Only generate charts for available scenarios
    available_scenarios = df['Scenario'].unique()
    
    if 'Same Key' in available_scenarios:
        plot_same_key_comparison(df)
    if 'Different Keys' in available_scenarios:
        plot_different_keys_comparison(df)
    if any('URI' in s for s in available_scenarios):
        plot_uri_hash_comparison(df)
    if 'Pool Size Scalability' in available_scenarios:
        plot_pool_size_scalability(df)
    if 'Concurrent Access' in available_scenarios:
        plot_concurrent_access(df)
    
    plot_comprehensive_comparison(df)
    
    # Add node removal comparison (for available removal data)
    if any('Removal' in s for s in available_scenarios):
        plot_node_removal_comparison(df)
    
    if 'Fixed Removals' in available_scenarios:
        plot_fixed_removals(df)
    if 'Progressive Removals' in available_scenarios:
        plot_progressive_removals(df)

    print("All charts generated successfully!")
    print("Files saved in current directory")


def main():
    parser = argparse.ArgumentParser(description='Generate benchmark comparison charts')
    parser.add_argument('--csv', default='benchmark_results.csv', help='CSV file with benchmark results')
    parser.add_argument('--chart', choices=['same-key', 'different-keys', 'uri-hash',
                                            'pool-size', 'concurrent', 'comprehensive',
                                            'fixed-removals', 'progressive-removals', 
                                            'node-removal', 'all'],
                        default='all', help='Chart to generate')

    args = parser.parse_args()

    # Load data
    df = load_data(args.csv)
    if df is None:
        return

    # Generate charts
    if args.chart == 'same-key':
        plot_same_key_comparison(df)
    elif args.chart == 'different-keys':
        plot_different_keys_comparison(df)
    elif args.chart == 'uri-hash':
        plot_uri_hash_comparison(df)
    elif args.chart == 'pool-size':
        plot_pool_size_scalability(df)
    elif args.chart == 'concurrent':
        plot_concurrent_access(df)
    elif args.chart == 'comprehensive':
        plot_comprehensive_comparison(df)
    elif args.chart == 'fixed-removals':
        plot_fixed_removals(df)
    elif args.chart == 'progressive-removals':
        plot_progressive_removals(df)
    elif args.chart == 'node-removal':
        plot_node_removal_comparison(df)
    elif args.chart == 'all':
        plot_all_charts(df)


if __name__ == '__main__':
    main()
