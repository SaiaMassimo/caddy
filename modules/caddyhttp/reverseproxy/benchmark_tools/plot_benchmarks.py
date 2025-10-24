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
    """Load benchmark data from CSV"""
    if not os.path.exists(csv_file):
        print(f"Error: {csv_file} not found!")
        return None
    
    df = pd.read_csv(csv_file)
    print(f"Loaded {len(df)} benchmark results from {csv_file}")
    return df

def plot_same_key_comparison(df):
    """Plot same key performance comparison"""
    same_key_data = df[df['Scenario'] == 'Same Key']
    
    if len(same_key_data) == 0:
        print("No same key data found")
        return
    
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
    
    # Normalize order: Rendezvous, BinomialHash, BinomialConsistent (if present)
    ordered_algs = [alg for alg in ['Rendezvous', 'Memento']
                    if alg in same_key_data['Algorithm'].values]
    times = [same_key_data[same_key_data['Algorithm'] == alg]['TimeNs'].iloc[0]
             for alg in ordered_algs]
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF'}
    colors = [color_map.get(alg, '#999999') for alg in ordered_algs]

    bars = ax1.bar(ordered_algs, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Same Key Performance Comparison')
    ax1.grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                f'{time:.1f} ns', ha='center', va='bottom', fontweight='bold')
    
    # Performance improvement chart (Rendezvous vs Binomial*, if present)
    rendezvous_time = same_key_data[same_key_data['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0]
    improvements = []
    labels = []
    improv_colors = []
    if 'Memento' in ordered_algs:
        mem_time = same_key_data[same_key_data['Algorithm'] == 'Memento']['TimeNs'].iloc[0]
        improvements.append(rendezvous_time / mem_time)
        labels.append('Memento')
        improv_colors.append(color_map['Memento'])

    bars2 = ax2.bar(labels, improvements, color=improv_colors, alpha=0.7, edgecolor='black')
    ax2.set_ylabel('Speedup Factor (vs Rendezvous)')
    ax2.set_title('Speedup over Rendezvous')
    ax2.grid(True, alpha=0.3)
    for bar, val in zip(bars2, improvements):
        ax2.text(bar.get_x() + bar.get_width()/2., val + max(val*0.01, 0.02), f'{val:.1f}x',
                 ha='center', va='bottom', fontweight='bold', fontsize=12)
    
    plt.tight_layout()
    plt.savefig('same_key_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()

def plot_different_keys_comparison(df):
    """Plot different keys performance comparison"""
    diff_keys_data = df[df['Scenario'] == 'Different Keys']
    
    if len(diff_keys_data) == 0:
        print("No different keys data found")
        return
    
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
    
    # Normalize order
    ordered_algs = [alg for alg in ['Rendezvous', 'Memento']
                    if alg in diff_keys_data['Algorithm'].values]
    times = [diff_keys_data[diff_keys_data['Algorithm'] == alg]['TimeNs'].iloc[0]
             for alg in ordered_algs]
    color_map = {'Rendezvous': '#FF6B6B', 'Memento': '#1E90FF'}
    colors = [color_map.get(alg, '#999999') for alg in ordered_algs]

    bars = ax1.bar(ordered_algs, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Different Keys Performance Comparison')
    ax1.grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                f'{time:.0f} ns', ha='center', va='bottom', fontweight='bold')
    
    # Performance improvement chart for any Binomial* present
    rendezvous_time = diff_keys_data[diff_keys_data['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0]
    improvements = []
    labels = []
    improv_colors = []
    if 'Memento' in ordered_algs:
        mem_time = diff_keys_data[diff_keys_data['Algorithm'] == 'Memento']['TimeNs'].iloc[0]
        improvements.append(rendezvous_time / mem_time)
        labels.append('Memento')
        improv_colors.append(color_map['Memento'])
    bars2 = ax2.bar(labels, improvements, color=improv_colors, alpha=0.7, edgecolor='black')
    ax2.set_ylabel('Speedup Factor (vs Rendezvous)')
    ax2.set_title('Speedup over Rendezvous')
    ax2.grid(True, alpha=0.3)
    for bar, val in zip(bars2, improvements):
        ax2.text(bar.get_x() + bar.get_width()/2., val + max(val*0.01, 0.02), f'{val:.1f}x', 
                 ha='center', va='bottom', fontweight='bold', fontsize=14)
    
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
    
    x = np.arange(2)
    width = 0.35
    
    same_times = [
        same_uri[same_uri['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0] if len(same_uri[same_uri['Algorithm'] == 'Rendezvous']) > 0 else 0,
        same_uri[same_uri['Algorithm'] == 'Memento']['TimeNs'].iloc[0] if len(same_uri[same_uri['Algorithm'] == 'Memento']) > 0 else 0
    ]
    diff_times = [
        diff_uri[diff_uri['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0] if len(diff_uri[diff_uri['Algorithm'] == 'Rendezvous']) > 0 else 0,
        diff_uri[diff_uri['Algorithm'] == 'Memento']['TimeNs'].iloc[0] if len(diff_uri[diff_uri['Algorithm'] == 'Memento']) > 0 else 0
    ]
    
    bars1 = ax.bar(x - width/2, same_times, width, label='Same URI', 
                  color='#FF6B6B', alpha=0.7)
    bars2 = ax.bar(x + width/2, diff_times, width, label='Different URIs', 
                  color='#4ECDC4', alpha=0.7)
    
    ax.set_xlabel('Algorithm')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('URI Hash Performance Comparison')
    ax.set_xticks(x)
    ax.set_xticklabels(['Rendezvous\nURIHash', 'Memento\nURI'])
    ax.legend()
    ax.grid(True, alpha=0.3)
    
    # Add value labels
    for bars in [bars1, bars2]:
        for bar in bars:
            height = bar.get_height()
            if height > 0:
                ax.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
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
                       xytext=(0,10), ha='center', fontweight='bold', fontsize=8)

    if rendezvous_sizes:
        ax.plot(rendezvous_sizes, rendezvous_times, marker='s', linewidth=3, markersize=8, 
                color='#FF6B6B', label='Rendezvous')
        
        # Add value labels for Rendezvous
        for x, y in zip(rendezvous_sizes, rendezvous_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points", 
                       xytext=(0,-15), ha='center', fontweight='bold', fontsize=8)
    
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
    
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
    
    # Bar chart
    algorithms = concurrent_data['Algorithm'].tolist()
    times = concurrent_data['TimeNs'].tolist()
    colors = ['#FF6B6B' if alg == 'Rendezvous' else '#1E90FF' for alg in algorithms]
    
    bars = ax1.bar(algorithms, times, color=colors, alpha=0.7, edgecolor='black')
    ax1.set_ylabel('Time (ns/op)')
    ax1.set_title('Concurrent Access Performance')
    ax1.grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, time in zip(bars, times):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                f'{time:.1f} ns', ha='center', va='bottom', fontweight='bold')
    
    # Performance improvement chart
    rendezvous_time = concurrent_data[concurrent_data['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0]
    memento_time = concurrent_data[concurrent_data['Algorithm'] == 'Memento']['TimeNs'].iloc[0]
    improvement = rendezvous_time / memento_time
    
    ax2.bar(['Performance\nImprovement'], [improvement], 
            color='#1E90FF', alpha=0.7, edgecolor='black')
    ax2.set_ylabel('Speedup Factor')
    ax2.set_title(f'Memento is {improvement:.1f}x Faster')
    ax2.grid(True, alpha=0.3)
    ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
            ha='center', va='bottom', fontweight='bold', fontsize=14)
    
    plt.tight_layout()
    plt.savefig('concurrent_access_comparison.png', dpi=300, bbox_inches='tight')
    plt.show()

def plot_comprehensive_comparison(df):
    """Plot comprehensive comparison across all scenarios"""
    fig, ax = plt.subplots(figsize=(16, 10))
    
    # Group data by scenario
    scenarios = df['Scenario'].unique()
    scenarios = [s for s in scenarios if s != 'Pool Size Scalability']  # Exclude scalability

    x = np.arange(len(scenarios))
    width = 0.35

    rendezvous_times = []
    memento_times = []

    for scenario in scenarios:
        scenario_data = df[df['Scenario'] == scenario]
        rendezvous_time = scenario_data[scenario_data['Algorithm'] == 'Rendezvous']['TimeNs'].iloc[0] if len(scenario_data[scenario_data['Algorithm'] == 'Rendezvous']) > 0 else 0
        memento_time = scenario_data[scenario_data['Algorithm'] == 'Memento']['TimeNs'].iloc[0] if len(scenario_data[scenario_data['Algorithm'] == 'Memento']) > 0 else 0

        rendezvous_times.append(rendezvous_time)
        memento_times.append(memento_time)

    bars1 = ax.bar(x - width, rendezvous_times, width, label='Rendezvous', 
                   color='#FF6B6B', alpha=0.7)
    bars2 = ax.bar(x, memento_times, width, label='Memento', 
                   color='#1E90FF', alpha=0.7)
    
    ax.set_xlabel('Scenario')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Comprehensive Performance Comparison: Rendezvous vs Memento')
    ax.set_xticks(x)
    ax.set_xticklabels(scenarios, rotation=45, ha='right')
    ax.legend()
    ax.grid(True, alpha=0.3)
    
    # Add value labels
    for bars in [bars1, bars2]:
        for bar in bars:
            height = bar.get_height()
            if height > 0:
                ax.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                       f'{height:.0f}', ha='center', va='bottom', fontweight='bold', fontsize=8)
    
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
                       xytext=(0,10), ha='center', fontweight='bold', fontsize=8)

    if rendezvous_sizes:
        ax.plot(rendezvous_sizes, rendezvous_times, marker='s', linewidth=3, markersize=8, 
                color='#FF6B6B', label='Rendezvous (5 unavailable)')
        
        for x, y in zip(rendezvous_sizes, rendezvous_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points", 
                       xytext=(0,-15), ha='center', fontweight='bold', fontsize=8)
    
    ax.set_xlabel('Total Pool Size')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Performance with 5 Nodes Removed/Unavailable')
    ax.grid(True, alpha=0.3)
    ax.legend()
    
    plt.tight_layout()
    plt.savefig('fixed_removals.png', dpi=300, bbox_inches='tight')
    plt.show()

def plot_progressive_removals(df):
    """Plot performance with progressive node removals"""
    removals_data = df[df['Scenario'] == 'Progressive Removals']
    
    if len(removals_data) == 0:
        print("No progressive removals data found")
        return
    
    fig, ax = plt.subplots(figsize=(12, 8))
    
    # Separate data by algorithm
    memento_data = removals_data[removals_data['Algorithm'] == 'Memento']
    rendezvous_data = removals_data[removals_data['Algorithm'] == 'Rendezvous']
    
    # Extract number of removed nodes
    memento_removed = []
    memento_times = []
    
    for _, row in memento_data.iterrows():
        test_name = row['TestName']
        if 'Removed' in test_name:
            # Extract number from format "Memento_100Nodes_50Removed"
            removed_str = test_name.split('Removed')[0].split('_')[-1]
            try:
                removed = int(removed_str)
                memento_removed.append(removed)
                memento_times.append(row['TimeNs'])
            except ValueError:
                continue
    
    rendezvous_removed = []
    rendezvous_times = []
    
    for _, row in rendezvous_data.iterrows():
        test_name = row['TestName']
        if 'Unavailable' in test_name:
            removed_str = test_name.split('Unavailable')[0].split('_')[-1]
            try:
                removed = int(removed_str)
                rendezvous_removed.append(removed)
                rendezvous_times.append(row['TimeNs'])
            except ValueError:
                continue
    
    # Sort by number of removed nodes
    if memento_removed:
        memento_sorted = sorted(zip(memento_removed, memento_times))
        memento_removed, memento_times = zip(*memento_sorted)
        
    if rendezvous_removed:
        rendezvous_sorted = sorted(zip(rendezvous_removed, rendezvous_times))
        rendezvous_removed, rendezvous_times = zip(*rendezvous_sorted)
    
    # Plot algorithms
    if memento_removed:
        ax.plot(memento_removed, memento_times, marker='o', linewidth=3, markersize=8, 
                color='#1E90FF', label='Memento')
        
        for x, y in zip(memento_removed, memento_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points", 
                       xytext=(0,10), ha='center', fontweight='bold', fontsize=8)

    if rendezvous_removed:
        ax.plot(rendezvous_removed, rendezvous_times, marker='s', linewidth=3, markersize=8, 
                color='#FF6B6B', label='Rendezvous')
        
        for x, y in zip(rendezvous_removed, rendezvous_times):
            ax.annotate(f'{y:.0f}', (x, y), textcoords="offset points", 
                       xytext=(0,-15), ha='center', fontweight='bold', fontsize=8)
    
    ax.set_xlabel('Number of Nodes Removed')
    ax.set_ylabel('Time (ns/op)')
    ax.set_title('Performance with Progressive Node Removals (100 total nodes)')
    ax.grid(True, alpha=0.3)
    ax.legend()
    
    plt.tight_layout()
    plt.savefig('progressive_removals.png', dpi=300, bbox_inches='tight')
    plt.show()

def plot_all_charts(df):
    """Generate all comparison charts"""
    print("Generating all benchmark comparison charts...")
    
    plot_same_key_comparison(df)
    plot_different_keys_comparison(df)
    plot_uri_hash_comparison(df)
    plot_pool_size_scalability(df)
    plot_concurrent_access(df)
    plot_comprehensive_comparison(df)
    plot_fixed_removals(df)
    plot_progressive_removals(df)
    
    print("All charts generated successfully!")
    print("Files saved:")
    print("- same_key_comparison.png")
    print("- different_keys_comparison.png")
    print("- uri_hash_comparison.png")
    print("- pool_size_scalability.png")
    print("- concurrent_access_comparison.png")
    print("- comprehensive_comparison.png")
    print("- fixed_removals.png")
    print("- progressive_removals.png")

def main():
    parser = argparse.ArgumentParser(description='Generate benchmark comparison charts')
    parser.add_argument('--csv', default='benchmark_results.csv', help='CSV file with benchmark results')
    parser.add_argument('--chart', choices=['same-key', 'different-keys', 'uri-hash', 
                                          'pool-size', 'concurrent', 'comprehensive', 
                                          'fixed-removals', 'progressive-removals', 'all'], 
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
    elif args.chart == 'all':
        plot_all_charts(df)

if __name__ == '__main__':
    main()
