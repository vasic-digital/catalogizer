// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
// Register and login a test user for subtitle API testing
const API_BASE = 'http://localhost:8080/api/v1';

async function registerTestUser() {
  const userData = {
    username: 'testuser',
    email: 'test@example.com',
    password: 'testpass123',
    first_name: 'Test',
    last_name: 'User'
  };

  const response = await fetch(`${API_BASE}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(userData)
  });

  if (response.ok) {
    console.log('✅ User registered successfully');
    return true;
  } else {
    const data = await response.json();
    console.log('❌ Registration failed:', data);
    return false;
  }
}

async function loginTestUser() {
  const loginData = {
    username: 'testuser',
    password: 'testpass123'
  };

  const response = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(loginData)
  });

  if (response.ok) {
    const data = await response.json();
    console.log('✅ Login successful');
    console.log('JWT Token:', data.token);
    return data.token;
  } else {
    const data = await response.json();
    console.log('❌ Login failed:', data);
    return null;
  }
}

async function testWithAuth(token) {
  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };

  // Test languages endpoint
  const response = await fetch(`${API_BASE}/subtitles/languages`, {
    headers
  });
  
  if (response.ok) {
    const data = await response.json();
    console.log('✅ Languages endpoint working with auth');
    console.log('Sample language:', data[0]);
  } else {
    console.log('❌ Languages endpoint failed even with auth');
  }

  // Test providers endpoint
  const providersResponse = await fetch(`${API_BASE}/subtitles/providers`, {
    headers
  });
  
  if (providersResponse.ok) {
    const data = await providersResponse.json();
    console.log('✅ Providers endpoint working with auth');
    console.log('Number of providers:', data.length);
  } else {
    console.log('❌ Providers endpoint failed even with auth');
  }
}

async function runAuthTests() {
  console.log('=== Setting up test authentication ===');
  
  // Try to register user (may already exist)
  await registerTestUser();
  
  // Login to get JWT token
  const token = await loginTestUser();
  
  if (token) {
    console.log('\n=== Testing with authentication ===');
    await testWithAuth(token);
  }
}

runAuthTests();