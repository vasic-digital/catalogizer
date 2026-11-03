// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
const http = require('http');

function makeRequest(options, data = null) {
  return new Promise((resolve, reject) => {
    const req = http.request(options, (res) => {
      let body = '';
      res.on('data', (chunk) => body += chunk);
      res.on('end', () => {
        try {
          const data = body ? JSON.parse(body) : {};
          resolve({
            status: res.statusCode,
            data: data
          });
        } catch (e) {
          resolve({
            status: res.statusCode,
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

async function testSimpleFlow() {
  try {
    console.log('=== Testing Health ===');
    const healthRes = await makeRequest({
      hostname: 'localhost',
      port: 8080,
      path: '/health',
      method: 'GET'
    });
    console.log('Health:', healthRes.status, healthRes.data);
    
    console.log('\n=== Testing Register ===');
    const registerRes = await makeRequest({
      hostname: 'localhost',
      port: 8080,
      path: '/api/v1/auth/register',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      }
    }, {
      username: 'testuser',
      email: 'test@example.com',
      password: 'password123',
      first_name: 'Test',
      last_name: 'User'
    });
    
    console.log('Register response:', registerRes.status, registerRes.data);
    
    console.log('\n=== Testing Login (with empty device_info) ===');
    const loginRes = await makeRequest({
      hostname: 'localhost',
      port: 8080,
      path: '/api/v1/auth/login',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      }
    }, {
      username: 'testuser',
      password: 'password123',
      device_info: {}
    });
    
    console.log('Login response:', loginRes.status, loginRes.data);
    
    if (loginRes.status === 200 && loginRes.data.data && loginRes.data.data.token) {
      const token = loginRes.data.data.token;
      
      console.log('\n=== Testing Subtitle Languages ===');
      const langRes = await makeRequest({
        hostname: 'localhost',
        port: 8080,
        path: '/api/v1/subtitles/languages',
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      
      console.log('Languages:', langRes.status, langRes.data);
      
      console.log('\n=== Success! ===');
    }
    
  } catch (error) {
    console.error('Error:', error);
  }
}

testSimpleFlow();