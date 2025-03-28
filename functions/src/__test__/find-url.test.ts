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
  'http://localhost:5001/test-project/us-central1/shortenUrl';

describe('shortenUrl Cloud Function response', () => {
  it('should return 201 status code for valid URL', async () => {
    const validLongUrl = 'https://www.google.com/very/long/path?query=string';
    const response = await axios.post<{ shortUrl: string }>(FUNCTION_URL, {
      longUrl: validLongUrl,
    });
    expect(response.status).toBe(201);
  });

  it('should return a shortUrl property in response', async () => {
    const validLongUrl = 'https://www.google.com/very/long/path?query=string';
    const response = await axios.post<{ shortUrl: string }>(FUNCTION_URL, {
      longUrl: validLongUrl,
    });
    expect(response.data).toHaveProperty('shortUrl');
  });

  it('should return a shortUrl matching the expected format', async () => {
    const validLongUrl = 'https://www.google.com/very/long/path?query=string';
    const response = await axios.post<{ shortUrl: string }>(FUNCTION_URL, {
      longUrl: validLongUrl,
    });
    expect(response.data.shortUrl).toMatch(
      /^https:\/\/dou\.la\/[A-Za-z0-9_-]{7}$/
    );
  });

  it('should return 405 status code for GET requests', async () => {
    try {
      await axios.get(FUNCTION_URL);
      throw new Error('Request should have failed with 405');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.status).toBe(405);
    }
  });

  it('should return "Method Not Allowed" message for GET requests', async () => {
    try {
      await axios.get(FUNCTION_URL);
      throw new Error('Request should have failed with 405');
    } catch (error) {
      const axiosError = error as AxiosError;
      expect(axiosError.response?.data).toBe('Method Not Allowed');
    }
  });

  it('should return 400 status code for incorrect Content-Type', async () => {
    try {
      await axios.post(FUNCTION_URL, 'plain text data', {
        headers: { 'Content-Type': 'text/plain' },
      });
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.status).toBe(400);
    }
  });

  it('should return appropriate error message for incorrect Content-Type', async () => {
    try {
      await axios.post(FUNCTION_URL, 'plain text data', {
        headers: { 'Content-Type': 'text/plain' },
      });
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.data.error).toContain(
        'Content-Type must be application/json'
      );
    }
  });

  it('should return 400 status code for missing longUrl', async () => {
    try {
      await axios.post(FUNCTION_URL, {});
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.status).toBe(400);
    }
  });

  it('should return "Invalid or missing URL" message for missing longUrl', async () => {
    try {
      await axios.post(FUNCTION_URL, {});
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.data.error).toBe('Invalid or missing URL');
    }
  });

  it('should return 400 status code for invalid longUrl', async () => {
    try {
      await axios.post(FUNCTION_URL, { longUrl: 'not-a-valid-url' });
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.status).toBe(400);
    }
  });

  it('should return "Invalid or missing URL" message for invalid longUrl', async () => {
    try {
      await axios.post(FUNCTION_URL, { longUrl: 'not-a-valid-url' });
      throw new Error('Request should have failed with 400');
    } catch (error) {
      const axiosError = error as AxiosError<{ error: string }>;
      expect(axiosError.response?.data.error).toBe('Invalid or missing URL');
    }
  });
});

describe('shortenUrl Firestore behavior', () => {
  it('should store the longUrl in Firestore', async () => {
    const { testEnv } = await setup();
    const { data } = await axios.post<{ shortUrl: string }>(FUNCTION_URL, {
      longUrl: 'https://www.google.com',
    });

    const shortCode = data.shortUrl.split('/').pop() as string;

    await testEnv.withSecurityRulesDisabled(async (context) => {
      const envRef = doc(context.firestore(), `urls/${shortCode}`);
      const envSnap = await getDoc(envRef);
      expect(envSnap.data()?.longUrl).toBe('https://www.google.com');
    });
  });

  it('should initialize clicks count to 0 in Firestore', async () => {
    const { testEnv } = await setup();
    const { data } = await axios.post<{ shortUrl: string }>(FUNCTION_URL, {
      longUrl: 'https://www.google.com',
    });

    const shortCode = data.shortUrl.split('/').pop() as string;

    await testEnv.withSecurityRulesDisabled(async (context) => {
      const envRef = doc(context.firestore(), `urls/${shortCode}`);
      const envSnap = await getDoc(envRef);
      expect(envSnap.data()?.clicks).toBe(0);
    });
  });
});
