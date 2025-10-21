#!/usr/bin/env python3
"""
Benchmark Plotter for Rendezvous vs Memento Comparison
Reads benchmark_results.csv and generates charts (does not execute benchmarks)
"""

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import argparse

class BenchmarkPlotter:
    def __init__(self, csv_path: str):
        self.df = pd.read_csv(csv_path)
        self.colors = {
            'rendezvous': '#FF6B6B',
            'memento': '#1E90FF'
        }
    
    def plot_same_key_comparison(self):
        """Plot comparison for same key scenario"""
        same_key = self.df[self.df['Scenario'] == 'Same Key']
        if same_key.empty:
            print('No data for Same Key')
            return
        rendezvous_time = float(same_key[same_key['Algorithm']=='Rendezvous']['TimeNs'].iloc[0]) if not same_key[same_key['Algorithm']=='Rendezvous'].empty else 0
        binomial_time = float(same_key[same_key['Algorithm']=='Memento']['TimeNs'].iloc[0]) if not same_key[same_key['Algorithm']=='Memento'].empty else 0
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for same key comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'Memento']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['memento']]
        
        bars = ax1.bar(algorithms, times, color=colors, alpha=0.7, edgecolor='black')
        ax1.set_ylabel('Time (ns/op)')
        ax1.set_title('Same Key Performance Comparison')
        ax1.grid(True, alpha=0.3)
        
        # Add value labels on bars
        for bar, time in zip(bars, times):
            height = bar.get_height()
            ax1.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                    f'{time:.1f} ns', ha='center', va='bottom', fontweight='bold')
        
        # Performance improvement chart
        improvement = rendezvous_time / binomial_time
        ax2.bar(['Performance\nImprovement'], [improvement], 
                color=self.colors['binomial'], alpha=0.7, edgecolor='black')
        ax2.set_ylabel('Speedup Factor')
        ax2.set_title(f'Memento is {improvement:.1f}x Faster')
        ax2.grid(True, alpha=0.3)
        ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
                ha='center', va='bottom', fontweight='bold', fontsize=14)
        
        plt.tight_layout()
        plt.savefig('same_key_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_different_keys_comparison(self):
        """Plot comparison for different keys scenario"""
        diff_keys = self.df[self.df['Scenario'] == 'Different Keys']
        if diff_keys.empty:
            print('No data for Different Keys')
            return
        rendezvous_time = float(diff_keys[diff_keys['Algorithm']=='Rendezvous']['TimeNs'].iloc[0]) if not diff_keys[diff_keys['Algorithm']=='Rendezvous'].empty else 0
        binomial_time = float(diff_keys[diff_keys['Algorithm']=='Memento']['TimeNs'].iloc[0]) if not diff_keys[diff_keys['Algorithm']=='Memento'].empty else 0
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for different keys comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'Memento']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['memento']]
        
        bars = ax1.bar(algorithms, times, color=colors, alpha=0.7, edgecolor='black')
        ax1.set_ylabel('Time (ns/op)')
        ax1.set_title('Different Keys Performance Comparison')
        ax1.grid(True, alpha=0.3)
        
        # Add value labels on bars
        for bar, time in zip(bars, times):
            height = bar.get_height()
            ax1.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                    f'{time:.0f} ns', ha='center', va='bottom', fontweight='bold')
        
        # Performance improvement chart
        improvement = rendezvous_time / binomial_time
        ax2.bar(['Performance\nImprovement'], [improvement], 
                color=self.colors['binomial'], alpha=0.7, edgecolor='black')
        ax2.set_ylabel('Speedup Factor')
        ax2.set_title(f'Memento is {improvement:.1f}x Faster')
        ax2.grid(True, alpha=0.3)
        ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
                ha='center', va='bottom', fontweight='bold', fontsize=14)
        
        plt.tight_layout()
        plt.savefig('different_keys_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_uri_hash_comparison(self):
        """Plot URI hash comparison"""
        uri_data = self.df[self.df['Scenario'].isin(['Same URI','Different URIs'])]
        if uri_data.empty:
            print('No data for URI hash comparison')
            return
        fig, ax = plt.subplots(figsize=(12, 8))
        
        rendezvous_same = float(uri_data[(uri_data['Algorithm']=='Rendezvous') & (uri_data['TestName'].str.contains('SameURI'))]['TimeNs'].iloc[0]) if not uri_data[(uri_data['Algorithm']=='Rendezvous') & (uri_data['TestName'].str.contains('SameURI'))].empty else 0
        binomial_same = float(uri_data[(uri_data['Algorithm']=='Memento') & (uri_data['TestName'].str.contains('SameURI'))]['TimeNs'].iloc[0]) if not uri_data[(uri_data['Algorithm']=='Memento') & (uri_data['TestName'].str.contains('SameURI'))].empty else 0
        rendezvous_diff = float(uri_data[(uri_data['Algorithm']=='Rendezvous') & (uri_data['TestName'].str.contains('DifferentURIs'))]['TimeNs'].iloc[0]) if not uri_data[(uri_data['Algorithm']=='Rendezvous') & (uri_data['TestName'].str.contains('DifferentURIs'))].empty else 0
        binomial_diff = float(uri_data[(uri_data['Algorithm']=='Memento') & (uri_data['TestName'].str.contains('DifferentURIs'))]['TimeNs'].iloc[0]) if not uri_data[(uri_data['Algorithm']=='Memento') & (uri_data['TestName'].str.contains('DifferentURIs'))].empty else 0
        
        # Prepare data for grouped bar chart
        x = np.arange(2)
        width = 0.35
        
        same_times = [rendezvous_same, binomial_same]
        diff_times = [rendezvous_diff, binomial_diff]
        
        bars1 = ax.bar(x - width/2, same_times, width, label='Same URI', 
                      color=self.colors['rendezvous'], alpha=0.7)
        bars2 = ax.bar(x + width/2, diff_times, width, label='Different URIs', 
                      color=self.colors['binomial'], alpha=0.7)
        
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
                ax.text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                       f'{height:.0f}', ha='center', va='bottom', fontweight='bold')
        
        plt.tight_layout()
        plt.savefig('uri_hash_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_pool_size_scalability(self):
        """Plot scalability with different pool sizes"""
        pool_data = self.df[self.df['Scenario'] == 'Pool Size Scalability']
        memento_rows = pool_data[pool_data['Algorithm']=='Memento']
        if memento_rows.empty:
            print('No data for pool size scalability')
            return
        fig, ax = plt.subplots(figsize=(12, 8))
        
        # Extract pool sizes and times
        pool_sizes = []
        times = []
        
        for _, row in memento_rows.iterrows():
            key = row['TestName']
            if 'PoolSize_' in key:
                size = int(key.split('PoolSize_')[1])
                pool_sizes.append(size)
                times.append(row['TimeNs'])
        
        # Sort by pool size
        sorted_data = sorted(zip(pool_sizes, times))
        pool_sizes, times = zip(*sorted_data)
        
        ax.plot(pool_sizes, times, marker='o', linewidth=3, markersize=8, 
                color=self.colors['memento'], label='Memento')
        ax.set_xlabel('Pool Size (Number of Upstreams)')
        ax.set_ylabel('Time (ns/op)')
        ax.set_title('Memento Scalability with Pool Size')
        ax.grid(True, alpha=0.3)
        ax.legend()
        
        # Add value labels on points
        for x, y in zip(pool_sizes, times):
            ax.annotate(f'{y:.0f} ns', (x, y), textcoords="offset points", 
                       xytext=(0,10), ha='center', fontweight='bold')
        
        plt.tight_layout()
        plt.savefig('pool_size_scalability.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_concurrent_access(self):
        """Plot concurrent access performance"""
        conc = self.df[self.df['Scenario'] == 'Concurrent Access']
        if conc.empty:
            print('No data for Concurrent Access')
            return
        rendezvous_time = float(conc[conc['Algorithm']=='Rendezvous']['TimeNs'].iloc[0]) if not conc[conc['Algorithm']=='Rendezvous'].empty else 0
        binomial_time = float(conc[conc['Algorithm']=='Memento']['TimeNs'].iloc[0]) if not conc[conc['Algorithm']=='Memento'].empty else 0
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for concurrent access comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'Memento']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['memento']]
        
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
        improvement = rendezvous_time / binomial_time
        ax2.bar(['Performance\nImprovement'], [improvement], 
                color=self.colors['binomial'], alpha=0.7, edgecolor='black')
        ax2.set_ylabel('Speedup Factor')
        ax2.set_title(f'Memento is {improvement:.1f}x Faster')
        ax2.grid(True, alpha=0.3)
        ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
                ha='center', va='bottom', fontweight='bold', fontsize=14)
        
        plt.tight_layout()
        plt.savefig('concurrent_access_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_all_comparisons(self):
        """Plot all benchmark comparisons"""
        print("Generating all benchmark comparison charts...")
        
        self.plot_same_key_comparison()
        self.plot_different_keys_comparison()
        self.plot_uri_hash_comparison()
        self.plot_pool_size_scalability()
        self.plot_concurrent_access()
        
        print("All charts generated successfully!")
        print("Files saved:")
        print("- same_key_comparison.png")
        print("- different_keys_comparison.png")
        print("- uri_hash_comparison.png")
        print("- pool_size_scalability.png")
        print("- concurrent_access_comparison.png")

def main():
    parser = argparse.ArgumentParser(description='Generate benchmark comparison charts from CSV')
    parser.add_argument('--csv', default='modules/caddyhttp/reverseproxy/benchmark_tools/benchmark_results.csv', help='Path to benchmark_results.csv')
    parser.add_argument('--chart', choices=['same-key', 'different-keys', 'uri-hash', 'pool-size', 'concurrent', 'all'], default='all', help='Chart to generate')
    
    args = parser.parse_args()
    
    plotter = BenchmarkPlotter(args.csv)
    
    if args.chart == 'same-key':
        plotter.plot_same_key_comparison()
    elif args.chart == 'different-keys':
        plotter.plot_different_keys_comparison()
    elif args.chart == 'uri-hash':
        plotter.plot_uri_hash_comparison()
    elif args.chart == 'pool-size':
        plotter.plot_pool_size_scalability()
    elif args.chart == 'concurrent':
        plotter.plot_concurrent_access()
    elif args.chart == 'all':
        plotter.plot_all_comparisons()

if __name__ == '__main__':
    main()
