// NOTE (registration policy): the server defaults to REGISTRATION_MODE=approval - a self-registered
// account is created inactive and the login that follows returns 403 "account pending approval"
// until an admin approves it. This ad-hoc debug script registers and then logs in, so it needs an
// API started with REGISTRATION_MODE=open (or the account approved by hand through
// PUT /api/v1/admin/users/:id {"status":"active"}). It is not part of any automated suite.
// Simple authentication test for subtitle endpoints
const API_BASE = 'http://localhost:8080/api/v1';

async function testAuthAndSubtitles() {
  console.log('=== Testing Authentication ===');

  // 1. Try to create a test user (may already exist)
  console.log('1. Creating test user...');
  const userData = {
    username: 'testuser',
    email: 'test@example.com',
    password: 'testpass123',
    first_name: 'Test',
    last_name: 'User'
  };

  let regResponse = await fetch(`${API_BASE}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(userData)
  });
  
  console.log('Registration status:', regResponse.status);
  if (regResponse.ok) {
    console.log('✅ User registered');
  } else {
    console.log('ℹ️ User may already exist');
  }

  // 2. Login
  console.log('\n2. Logging in...');
  const loginData = {
    username: 'testuser',
    password: 'testpass123'
  };

  const loginResponse = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(loginData)
  });

  if (loginResponse.ok) {
    const loginResult = await loginResponse.json();
    console.log('✅ Login successful');
    console.log('Session token:', loginResult.session_token ? 'Present' : 'Missing');
    return loginResult.session_token;
  } else {
    const error = await loginResponse.json();
    console.log('❌ Login failed:', error);
    return null;
  }
}

async function testSubtitleEndpoints(token) {
  console.log('\n=== Testing Subtitle Endpoints ===');
  const headers = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };

  // Test 1: Get languages
  console.log('1. Testing languages endpoint...');
  const langResponse = await fetch(`${API_BASE}/subtitles/languages`, { headers });
  console.log(`Status: ${langResponse.status}`);
  
  if (langResponse.ok) {
    const langs = await langResponse.json();
    console.log(`✅ Languages working (${langs.length} languages)`);
    console.log('Sample:', langs[0]);
  } else {
    console.log('❌ Languages failed');
  }

  // Test 2: Get providers
  console.log('\n2. Testing providers endpoint...');
  const provResponse = await fetch(`${API_BASE}/subtitles/providers`, { headers });
  console.log(`Status: ${provResponse.status}`);
  
  if (provResponse.ok) {
    const provs = await provResponse.json();
    console.log(`✅ Providers working (${provs.length} providers)`);
    console.log('Sample:', provs[0]);
  } else {
    console.log('❌ Providers failed');
  }

  // Test 3: Search subtitles
  console.log('\n3. Testing search endpoint...');
  const searchResponse = await fetch(`${API_BASE}/subtitles/search?query=test`, { headers });
  console.log(`Status: ${searchResponse.status}`);
  
  if (searchResponse.ok) {
    const searchResult = await searchResponse.json();
    console.log(`✅ Search working (${searchResult.total} results)`);
  } else {
    console.log('❌ Search failed');
  }
}

async function runCompleteTest() {
  const token = await testAuthAndSubtitles();
  if (token) {
    await testSubtitleEndpoints(token);
  } else {
    console.log('Cannot test subtitle endpoints without authentication');
  }
}

runCompleteTest();