// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
// Debug authentication by checking detailed error responses
const API_BASE = 'http://localhost:8080/api/v1';

async function testAuthFlow() {
  console.log('=== Testing authentication flow ===');

  // First check if we need to initialize
  console.log('1. Checking init status...');
  const initResponse = await fetch(`${API_BASE}/auth/init-status`);
  const initData = await initResponse.json();
  console.log('Init status:', initData);

  // If no admin user, create one
  if (initData.has_admin === false) {
    console.log('No admin user exists, creating one...');
    const adminData = {
      username: 'admin',
      email: 'admin@example.com',
      password: 'adminpass123',
      first_name: 'Admin',
      last_name: 'User'
    };

    const adminResponse = await fetch(`${API_BASE}/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(adminData)
    });
    
    const adminResult = await adminResponse.json();
    console.log('Admin registration result:', adminResult);
  }

  // Now try to login with existing credentials
  console.log('2. Attempting login...');
  const loginData = {
    username: 'admin',
    password: 'adminpass123'
  };

  const loginResponse = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(loginData)
  });

  const loginResult = await loginResponse.json();
  console.log('Login result:', loginResult);

  if (loginResult.token) {
    console.log('✅ Authentication successful!');
    console.log('JWT Token:', loginResult.token);
    return loginResult.token;
  } else {
    console.log('❌ Authentication failed');
    return null;
  }
}

async function testSubtitleEndpoints(token) {
  console.log('\n=== Testing subtitle endpoints with auth ===');
  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };

  // Test languages
  console.log('Testing /subtitles/languages...');
  const langResponse = await fetch(`${API_BASE}/subtitles/languages`, { headers });
  console.log('Status:', langResponse.status);
  if (langResponse.ok) {
    const langData = await langResponse.json();
    console.log('✅ Languages endpoint working');
    console.log('First language:', langData[0]);
  } else {
    const error = await langResponse.json();
    console.log('❌ Languages endpoint failed:', error);
  }

  // Test providers
  console.log('\nTesting /subtitles/providers...');
  const provResponse = await fetch(`${API_BASE}/subtitles/providers`, { headers });
  console.log('Status:', provResponse.status);
  if (provResponse.ok) {
    const provData = await provResponse.json();
    console.log('✅ Providers endpoint working');
    console.log('Number of providers:', provData.length);
  } else {
    const error = await provResponse.json();
    console.log('❌ Providers endpoint failed:', error);
  }

  // Test search
  console.log('\nTesting /subtitles/search...');
  const searchResponse = await fetch(`${API_BASE}/subtitles/search?query=test`, { headers });
  console.log('Status:', searchResponse.status);
  if (searchResponse.ok) {
    const searchData = await searchResponse.json();
    console.log('✅ Search endpoint working');
    console.log('Results count:', searchData.total || 0);
  } else {
    const error = await searchResponse.json();
    console.log('❌ Search endpoint failed:', error);
  }
}

async function runDebugTests() {
  const token = await testAuthFlow();
  if (token) {
    await testSubtitleEndpoints(token);
  }
}

runDebugTests();