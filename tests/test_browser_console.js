// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
// Test subtitle management via browser console
// This script can be pasted directly into the browser console on localhost:3000

async function testSubtitleEndpoints() {
  const baseURL = 'http://localhost:8080';
  
  try {
    // Step 1: Try to register a test user
    console.log('=== Registering test user ===');
    const registerResponse = await fetch(`${baseURL}/api/v1/auth/register`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        username: 'testuser',
        email: 'test@example.com',
        password: 'password123',
        role_id: 1, // Assuming admin role is 1
        device_info: {
          device_type: 'desktop',
          platform: 'web',
          platform_version: navigator.userAgent,
          app_version: '1.0.0'
        }
      })
    });
    
    if (registerResponse.ok) {
      console.log('User registered successfully');
      console.log(await registerResponse.json());
    } else {
      console.log('Registration failed, user might already exist');
    }
    
    // Step 2: Try to login with complete device info
    console.log('\n=== Logging in with device info ===');
    const loginData = {
      username: 'testuser',
      password: 'password123',
      device_info: {
        device_type: 'desktop',
        platform: 'web',
        platform_version: navigator.userAgent,
        app_version: '1.0.0',
        device_name: 'Test Browser'
      },
      remember_me: false
    };
    
    console.log('Login request:', JSON.stringify(loginData, null, 2));
    
    const loginResponse = await fetch(`${baseURL}/api/v1/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(loginData)
    });
    
    if (!loginResponse.ok) {
      throw new Error(`Login failed: ${loginResponse.status} - ${await loginResponse.text()}`);
    }
    
    const loginResult = await loginResponse.json();
    console.log('Login successful:', loginResult);
    
    const token = loginResult.data?.token || loginResult.session_token;
    if (!token) {
      throw new Error('No token in login response');
    }
    
    console.log('Got token:', token.substring(0, 50) + '...');
    
    // Step 3: Test subtitle endpoints
    console.log('\n=== Testing subtitle endpoints ===');
    
    const headers = {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    };
    
    // Test languages endpoint
    const languagesResponse = await fetch(`${baseURL}/api/v1/subtitles/languages`, {
      headers
    });
    console.log('Languages response:', await languagesResponse.json());
    
    // Test providers endpoint
    const providersResponse = await fetch(`${baseURL}/api/v1/subtitles/providers`, {
      headers
    });
    console.log('Providers response:', await providersResponse.json());
    
    // Test search endpoint
    const searchResponse = await fetch(`${baseURL}/api/v1/subtitles/search?query=test&language=en`, {
      headers
    });
    console.log('Search response:', await searchResponse.json());
    
    console.log('\n=== All tests completed successfully! ===');
    return token;
    
  } catch (error) {
    console.error('Test failed:', error);
  }
}

// Run the test
console.log('Starting subtitle management test...');
testSubtitleEndpoints();