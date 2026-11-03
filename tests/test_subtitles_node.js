// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
const http = require('http');
const https = require('https');

function makeRequest(url, data = null, method = 'GET', headers = {}) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith('https') ? https : http;
    const urlObj = new URL(url);
    
    const options = {
      hostname: urlObj.hostname,
      port: urlObj.port,
      path: urlObj.pathname + urlObj.search,
      method: method,
      headers: {
        'Content-Type': 'application/json',
        ...headers
      }
    };

    const req = client.request(options, (res) => {
      let body = '';
      res.on('data', (chunk) => body += chunk);
      res.on('end', () => {
        try {
          const data = body ? JSON.parse(body) : {};
          resolve({
            status: res.statusCode,
            headers: res.headers,
            data: data
          });
        } catch (e) {
          resolve({
            status: res.statusCode,
            headers: res.headers,
            data: body
          });
        }
      });
    });

    req.on('error', reject);

    if (data) {
      req.write(JSON.stringify(data));
    }
    req.end();
  });
}

async function testSubtitleEndpoints() {
  const baseURL = 'http://localhost:8080';
  
  try {
    console.log('=== Testing Server Health ===');
    const healthResponse = await makeRequest(`${baseURL}/health`);
    console.log('Health check:', healthResponse.status, healthResponse.data);
    
    console.log('\n=== Registering Test User ===');
    const registerData = {
      username: 'testuser',
      email: 'test@example.com',
      password: 'password123',
      role_id: 1,
      first_name: 'Test',
      last_name: 'User',
      device_info: {
        device_type: 'desktop',
        platform: 'web',
        platform_version: '1.0.0',
        app_version: '1.0.0'
      }
    };
    
    const registerResponse = await makeRequest(`${baseURL}/api/v1/auth/register`, registerData, 'POST');
    console.log('Register response:', registerResponse.status, registerResponse.data);
    
    console.log('\n=== Logging In ===');
    const loginData = {
      username: 'testuser',
      password: 'password123',
      device_info: {
        device_type: 'desktop',
        platform: 'web',
        platform_version: '1.0.0',
        app_version: '1.0.0',
        device_name: 'TestEnvironment'
      },
      remember_me: false
    };
    
    const loginResponse = await makeRequest(`${baseURL}/api/v1/auth/login`, loginData, 'POST');
    console.log('Login response:', loginResponse.status, loginResponse.data);
    
    if (loginResponse.status !== 200) {
      throw new Error(`Login failed: ${loginResponse.status} - ${JSON.stringify(loginResponse.data)}`);
    }
    
    const token = loginResponse.data.data?.token || loginResponse.data.session_token;
    if (!token) {
      console.error('No token in response, available keys:', Object.keys(loginResponse.data));
      throw new Error('No authentication token found');
    }
    
    console.log(`Got token: ${token.substring(0, 50)}...`);
    
    console.log('\n=== Testing Subtitle Endpoints ===');
    const authHeaders = {
      'Authorization': `Bearer ${token}`
    };
    
    // Test languages
    console.log('Testing /api/v1/subtitles/languages...');
    const langResponse = await makeRequest(`${baseURL}/api/v1/subtitles/languages`, null, 'GET', authHeaders);
    console.log('Languages:', langResponse.status, langResponse.data);
    
    // Test providers
    console.log('Testing /api/v1/subtitles/providers...');
    const provResponse = await makeRequest(`${baseURL}/api/v1/subtitles/providers`, null, 'GET', authHeaders);
    console.log('Providers:', provResponse.status, provResponse.data);
    
    // Test search
    console.log('Testing /api/v1/subtitles/search?query=test&language=en...');
    const searchResponse = await makeRequest(`${baseURL}/api/v1/subtitles/search?query=test&language=en`, null, 'GET', authHeaders);
    console.log('Search:', searchResponse.status, searchResponse.data);
    
    console.log('\n=== All Tests Completed Successfully! ===');
    
  } catch (error) {
    console.error('Test failed:', error.message);
    console.error(error.stack);
  }
}

// Run the tests
testSubtitleEndpoints();