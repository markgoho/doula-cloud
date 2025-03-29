import {
  initializeTestEnvironment,
  RulesTestEnvironment,
} from '@firebase/rules-unit-testing';
import axios, { type AxiosError } from 'axios';
import { doc, getDoc } from 'firebase/firestore';

interface SetupConfig {
  projectId: string;
}

const DEFAULT_SETUP: SetupConfig = { projectId: 'test-project' };

async function setup({ projectId }: SetupConfig = DEFAULT_SETUP) {
  const testEnv: RulesTestEnvironment = await initializeTestEnvironment({
    firestore: {
      port: 8080,
      host: 'localhost',
    },
    projectId,
  });

  return { testEnv };
}

const FUNCTION_URL =
  'http://localhost:5001/test-project/us-central1/redirectUrl';

describe('redirectUrl Cloud Function response', () => {
  it('should return 302 status code for root path', async () => {
    try {
      await axios.get(FUNCTION_URL, { maxRedirects: 0 });
      throw new Error('Request should have failed');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.status).toBe(302);
    }
  });

  it('should set location header to doula.cloud for root path', async () => {
    try {
      await axios.get(FUNCTION_URL, { maxRedirects: 0 });
      throw new Error('Request should have failed');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.headers.location).toBe('https://doula.cloud');
    }
  });

  it('should return 404 status code for non-existent shortCode', async () => {
    try {
      await axios.get(`${FUNCTION_URL}/nonexistent`);
      throw new Error('Request should have failed');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.status).toBe(404);
    }
  });

  it('should return "Short URL not found" message for non-existent shortCode', async () => {
    try {
      await axios.get(`${FUNCTION_URL}/nonexistent`);
      throw new Error('Request should have failed');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.data).toBe('Short URL not found.');
    }
  });

  it('should return 302 status code for valid shortCode', async () => {
    // First create a URL to get a valid shortCode
    const { data: createData } = await axios.post<{ shortUrl: string }>(
      'http://localhost:5001/test-project/us-central1/shortenUrl',
      { longUrl: 'https://www.google.com' }
    );

    const shortCode = createData.shortUrl.split('/').pop() as string;

    try {
      await axios.get(`${FUNCTION_URL}/${shortCode}`, { maxRedirects: 0 });
      throw new Error('Request should have failed');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.status).toBe(302);
    }
  });
});

describe('redirectUrl Firestore behavior', () => {
  it('should increment clicks count in Firestore after redirect', async () => {
    const { testEnv } = await setup();

    // First create a URL to get a valid shortCode
    const { data: createData } = await axios.post<{ shortUrl: string }>(
      'http://localhost:5001/test-project/us-central1/shortenUrl',
      { longUrl: 'https://www.google.com' }
    );

    const shortCode = createData.shortUrl.split('/').pop() as string;

    // Perform the redirect
    try {
      await axios.get(`${FUNCTION_URL}/${shortCode}`, { maxRedirects: 0 });
      throw new Error('Request should have failed');
    } catch {
      // Expected redirect, continue with verification
    }

    // Add a small delay to allow Firestore to update
    // await new Promise((resolve) => setTimeout(resolve, 1000));

    // Verify the clicks were incremented
    await testEnv.withSecurityRulesDisabled(async (context) => {
      const envRef = doc(context.firestore(), `urls/${shortCode}`);
      const envSnap = await getDoc(envRef);
      expect(envSnap.data()?.clicks).toBe(1);
    });
  });
});
