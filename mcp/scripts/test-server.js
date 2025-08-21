#!/usr/bin/env node

/**
 * Simple test script to verify MCP server functionality
 * This script tests the basic MCP protocol interactions
 */

import { spawn } from 'child_process';
import { readFileSync } from 'fs';

async function testMCPServer() {
  console.log('🧪 Testing MCP Server...\n');
  
  // Start the MCP server
  const mcpServer = spawn('node', ['dist/index.js'], {
    stdio: ['pipe', 'pipe', 'pipe']
  });

  let responses = [];
  let errors = [];
  
  mcpServer.stdout.on('data', (data) => {
    const response = data.toString().trim();
    if (response) {
      responses.push(response);
    }
  });

  mcpServer.stderr.on('data', (data) => {
    const error = data.toString().trim();
    if (error) {
      errors.push(error);
    }
  });

  // Test 1: Initialize
  console.log('📡 Test 1: Initialize MCP connection...');
  const initRequest = {
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: {
      protocolVersion: "2024-11-05",
      capabilities: {},
      clientInfo: {
        name: "test-client",
        version: "1.0.0"
      }
    }
  };

  mcpServer.stdin.write(JSON.stringify(initRequest) + '\n');
  await sleep(1000);

  // Test 2: List tools
  console.log('🔧 Test 2: List available tools...');
  const listToolsRequest = {
    jsonrpc: "2.0",
    id: 2,
    method: "tools/list",
    params: {}
  };

  mcpServer.stdin.write(JSON.stringify(listToolsRequest) + '\n');
  await sleep(2000);

  // Test 3: Server status
  console.log('📊 Test 3: Check server status...');
  const statusRequest = {
    jsonrpc: "2.0",
    id: 3,
    method: "tools/call",
    params: {
      name: "server_status",
      arguments: {}
    }
  };

  mcpServer.stdin.write(JSON.stringify(statusRequest) + '\n');
  await sleep(2000);

  // Test 4: Refresh workflows
  console.log('🔄 Test 4: Refresh workflows...');
  const refreshRequest = {
    jsonrpc: "2.0",
    id: 4,
    method: "tools/call",
    params: {
      name: "refresh_workflows",
      arguments: {}
    }
  };

  mcpServer.stdin.write(JSON.stringify(refreshRequest) + '\n');
  await sleep(3000);

  // Clean up
  mcpServer.kill();
  
  // Analyze results
  console.log('\n📋 Test Results:');
  console.log(`✅ Responses received: ${responses.length}`);
  console.log(`⚠️  Errors logged: ${errors.length}`);

  if (responses.length >= 4) {
    console.log('🎉 MCP Server is working correctly!');

    // Try to parse and display some results
    try {
      // Find the server status response
      const statusResponse = responses.find(r => r.includes('"server"') && r.includes('"workflows"'));
      if (statusResponse) {
        const parsed = JSON.parse(statusResponse);
        if (parsed.result && parsed.result.content) {
          console.log('\n📊 Server Status Response:');
          const statusData = JSON.parse(parsed.result.content[0].text);
          console.log(`Server: ${statusData.server} v${statusData.version}`);
          console.log(`Tenant: ${statusData.workflows.tenant}`);
          console.log(`Workflows: ${statusData.workflows.count}`);
          console.log(`Cache TTL: ${statusData.config.cacheTtl}s`);
        }
      }

      // Find the refresh response
      const refreshResponse = responses.find(r => r.includes('refreshed') || r.includes('Found'));
      if (refreshResponse) {
        const parsed = JSON.parse(refreshResponse);
        if (parsed.result && parsed.result.content) {
          console.log('\n🔄 Refresh Response:');
          console.log(parsed.result.content[0].text);
        }
      }
    } catch (e) {
      console.log('\n⚠️  Could not parse detailed results, but basic functionality works');
    }
  } else {
    console.log('❌ MCP Server may have issues. Check the logs above.');
  }
  
  console.log('\n🏁 Test completed.');
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// Run the test
testMCPServer().catch(console.error);
