const axios = require('axios');

// Test authentication with proper DeviceInfo structure
async function testAuth() {
  const baseURL = 'http://localhost:8080';
  
  try {
    // First check what endpoints are available
    console.log('=== Checking server status ===');
    const healthResponse = await axios.get(`${baseURL}/health`);
    console.log('Server health:', healthResponse.data);
    
    console.log('\n=== Testing authentication ===');
    
    // Try login with minimal required fields
    const loginData = {
      username: 'admin',
      password: process.env.ADMIN_PASSWORD
      // DeviceInfo is optional, don't send it to avoid binding issues
    };
    
    console.log('Login request data:', JSON.stringify(loginData, null, 2));
    
    const loginResponse = await axios.post(`${baseURL}/api/v1/auth/login`, loginData, {
      headers: {
        'Content-Type': 'application/json'
      }
    });
    
    console.log('Login successful!');
    console.log('Response:', JSON.stringify(loginResponse.data, null, 2));
    
    if (loginResponse.data.data && loginResponse.data.data.token) {
      const token = loginResponse.data.data.token;
      
      console.log('\n=== Testing authenticated endpoint ===');
      const authResponse = await axios.get(`${baseURL}/api/v1/auth/me`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      
      console.log('Authenticated user data:', JSON.stringify(authResponse.data, null, 2));
      
      console.log('\n=== Testing subtitle endpoint ===');
      const subtitleResponse = await axios.get(`${baseURL}/api/v1/subtitles/languages`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      
      console.log('Subtitle languages:', JSON.stringify(subtitleResponse.data, null, 2));
      
      return token;
    }
    
  } catch (error) {
    console.error('Error:', error.response ? error.response.data : error.message);
    if (error.response) {
      console.error('Status:', error.response.status);
      console.error('Headers:', error.response.headers);
    }
  }
}

testAuth();