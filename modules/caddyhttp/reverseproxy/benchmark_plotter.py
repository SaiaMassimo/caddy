#!/usr/bin/env python3
"""
Benchmark Plotter for Rendezvous vs BinomialHash Comparison
Generates visual charts from benchmark results
"""

import matplotlib.pyplot as plt
import numpy as np
import subprocess
import re
import json
from typing import Dict, List, Tuple
import argparse

class BenchmarkPlotter:
    def __init__(self):
        self.results = {}
        self.colors = {
            'rendezvous': '#FF6B6B',
            'binomial': '#4ECDC4'
        }
        
    def run_benchmark(self, pattern: str) -> Dict[str, float]:
        """Run benchmark and parse results"""
        try:
            result = subprocess.run([
                'go', 'test', './modules/caddyhttp/reverseproxy', 
                '-bench', pattern, '-benchmem'
            ], capture_output=True, text=True, cwd='/home/massimo.saia/sw/caddy')
            
            if result.returncode != 0:
                print(f"Error running benchmark: {result.stderr}")
                return {}
                
            return self.parse_benchmark_output(result.stdout)
        except Exception as e:
            print(f"Error: {e}")
            return {}
    
    def parse_benchmark_output(self, output: str) -> Dict[str, float]:
        """Parse benchmark output and extract timing data"""
        results = {}
        lines = output.split('\n')
        
        for line in lines:
            if 'Benchmark' in line and 'ns/op' in line:
                # Extract benchmark name and timing
                match = re.search(r'Benchmark([^-]+)-(\d+)\s+(\d+)\s+([\d.]+)\s+ns/op', line)
                if match:
                    name = match.group(1)
                    timing = float(match.group(4))
                    results[name] = timing
                    
        return results
    
    def plot_same_key_comparison(self):
        """Plot comparison for same key scenario"""
        results = self.run_benchmark('BenchmarkRendezvousVsBinomial_SameKey')
        
        if not results:
            print("No results found for same key comparison")
            return
            
        rendezvous_time = results.get('Rendezvous_IPHash_SameKey', 0)
        binomial_time = results.get('Binomial_SameKey', 0)
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for same key comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'BinomialHash']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['binomial']]
        
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
        ax2.set_title(f'BinomialHash is {improvement:.1f}x Faster')
        ax2.grid(True, alpha=0.3)
        ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
                ha='center', va='bottom', fontweight='bold', fontsize=14)
        
        plt.tight_layout()
        plt.savefig('same_key_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_different_keys_comparison(self):
        """Plot comparison for different keys scenario"""
        results = self.run_benchmark('BenchmarkRendezvousVsBinomial_DifferentKeys')
        
        if not results:
            print("No results found for different keys comparison")
            return
            
        rendezvous_time = results.get('Rendezvous_IPHash_DifferentKeys', 0)
        binomial_time = results.get('Binomial_DifferentKeys', 0)
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for different keys comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'BinomialHash']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['binomial']]
        
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
        ax2.set_title(f'BinomialHash is {improvement:.1f}x Faster')
        ax2.grid(True, alpha=0.3)
        ax2.text(0, improvement + improvement*0.01, f'{improvement:.1f}x', 
                ha='center', va='bottom', fontweight='bold', fontsize=14)
        
        plt.tight_layout()
        plt.savefig('different_keys_comparison.png', dpi=300, bbox_inches='tight')
        plt.show()
        
    def plot_uri_hash_comparison(self):
        """Plot URI hash comparison"""
        results = self.run_benchmark('BenchmarkRendezvousVsBinomial_URIHash')
        
        if not results:
            print("No results found for URI hash comparison")
            return
            
        fig, ax = plt.subplots(figsize=(12, 8))
        
        # Extract data
        rendezvous_same = results.get('Rendezvous_URIHash_SameURI', 0)
        binomial_same = results.get('Binomial_URI_SameURI', 0)
        rendezvous_diff = results.get('Rendezvous_URIHash_DifferentURIs', 0)
        binomial_diff = results.get('Binomial_URI_DifferentURIs', 0)
        
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
        ax.set_xticklabels(['Rendezvous\nURIHash', 'BinomialHash\nURI'])
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
        results = self.run_benchmark('BenchmarkRendezvousVsBinomial_DifferentPoolSizes')
        
        if not results:
            print("No results found for pool size scalability")
            return
            
        fig, ax = plt.subplots(figsize=(12, 8))
        
        # Extract pool sizes and times
        pool_sizes = []
        times = []
        
        for key, time in results.items():
            if 'PoolSize' in key:
                # Extract pool size from key like "Binomial_PoolSize_3"
                size_match = re.search(r'PoolSize_(\d+)', key)
                if size_match:
                    size = int(size_match.group(1))
                    pool_sizes.append(size)
                    times.append(time)
        
        # Sort by pool size
        sorted_data = sorted(zip(pool_sizes, times))
        pool_sizes, times = zip(*sorted_data)
        
        ax.plot(pool_sizes, times, marker='o', linewidth=3, markersize=8, 
                color=self.colors['binomial'], label='BinomialHash')
        ax.set_xlabel('Pool Size (Number of Upstreams)')
        ax.set_ylabel('Time (ns/op)')
        ax.set_title('BinomialHash Scalability with Pool Size')
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
        results = self.run_benchmark('BenchmarkRendezvousVsBinomial_ConcurrentAccess')
        
        if not results:
            print("No results found for concurrent access comparison")
            return
            
        rendezvous_time = results.get('Rendezvous_IPHash_Concurrent', 0)
        binomial_time = results.get('Binomial_Concurrent', 0)
        
        if rendezvous_time == 0 or binomial_time == 0:
            print("Missing data for concurrent access comparison")
            return
            
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
        
        # Bar chart
        algorithms = ['Rendezvous\nIPHash', 'BinomialHash']
        times = [rendezvous_time, binomial_time]
        colors = [self.colors['rendezvous'], self.colors['binomial']]
        
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
        ax2.set_title(f'BinomialHash is {improvement:.1f}x Faster')
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
    parser = argparse.ArgumentParser(description='Generate benchmark comparison charts')
    parser.add_argument('--chart', choices=['same-key', 'different-keys', 'uri-hash', 
                                          'pool-size', 'concurrent', 'all'], 
                       default='all', help='Chart to generate')
    
    args = parser.parse_args()
    
    plotter = BenchmarkPlotter()
    
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
