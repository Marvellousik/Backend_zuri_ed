# Zuri Mobile Integration Guide (React Native)

> How to connect a React Native app to the existing Zuri backend.

---

## Architecture

Your mobile app is just **another client** - identical role to the Next.js frontend. It talks to the same API Gateway at `:8080` (or your production domain).

```
React Native App
  |-- SecureStore (tokens)
  |-- AsyncStorage (cache)
  |-- fetch / axios
  v
API Gateway (:8080)
  v
Go Services + Python AI Services
```

**Nothing in the backend needs to change.** The Gateway already handles:
- JWT validation
- Rate limiting
- CORS (you will send `Origin` header from mobile)
- Daily AI quotas

---

## 1. Project Setup

```bash
npx react-native@latest init ZuriMobile
# or
npx create-expo-app ZuriMobile

# Required dependencies
npm install @react-native-async-storage/async-storage
npm install expo-secure-store        # or react-native-keychain
npm install @stomp/stompjs ws        # WebSocket
npm install react-native-document-picker
npm install react-native-audio-recorder-player
npm install react-native-fs
npm install axios
npm install zustand
npm install react-query @tanstack/react-query
```

---

## 2. Environment Config

Create `src/config/api.ts`:

```typescript
export const API_CONFIG = {
  // Development
  BASE_URL: __DEV__ ? 'http://10.0.2.2:8080' : 'https://api.zuri.app',
  
  // Android emulator uses 10.0.2.2 to reach host localhost
  // iOS simulator uses localhost directly
  // Physical device uses your machine's LAN IP (e.g., 192.168.1.100:8080)
  
  TIMEOUT: 30000,
  AI_TIMEOUT: 300000,
} as const;
```

**Important for physical device testing:**
- Backend must bind to `0.0.0.0` (not `127.0.0.1`)
- Use your machine's LAN IP: `http://192.168.1.xxx:8080`
- Or use ngrok: `ngrok http 8080`

---

## 3. Secure Token Storage

Replace the web `localStorage` with `expo-secure-store` (or `react-native-keychain`).

`src/store/secureStorage.ts`:

```typescript
import * as SecureStore from 'expo-secure-store';

export const secureStorage = {
  async getItem(key: string): Promise<string | null> {
    try {
      return await SecureStore.getItemAsync(key);
    } catch {
      return null;
    }
  },
  
  async setItem(key: string, value: string): Promise<void> {
    await SecureStore.setItemAsync(key, value);
  },
  
  async removeItem(key: string): Promise<void> {
    await SecureStore.deleteItemAsync(key);
  },
};
```

**Why not AsyncStorage for tokens?** AsyncStorage is unencrypted. Tokens should use SecureStore / Keychain.

---

## 4. Auth Store (Zustand)

Port the web auth store to React Native. Only the **persistence layer** changes.

`src/store/authStore.ts`:

```typescript
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { secureStorage } from './secureStorage';

interface Tokens {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
}

interface User {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  email_verified: boolean;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  accessToken: string | null;
  refreshToken: string | null;
  tokenExpiresAt: string | null;
  
  login: (user: User, tokens: Tokens) => void;
  logout: () => Promise<void>;
  refreshAccessToken: () => Promise<boolean>;
  isTokenExpired: () => boolean;
  setTokens: (tokens: Tokens) => void;
}

// Store sensitive tokens in SecureStore
const tokenStorage = {
  getItem: async (name: string): Promise<string | null> => {
    return secureStorage.getItem(name);
  },
  setItem: async (name: string, value: string): Promise<void> => {
    return secureStorage.setItem(name, value);
  },
  removeItem: async (name: string): Promise<void> => {
    return secureStorage.removeItem(name);
  },
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      isAuthenticated: false,
      isLoading: true,
      accessToken: null,
      refreshToken: null,
      tokenExpiresAt: null,
      
      login: (user, tokens) =>
        set({
          user,
          isAuthenticated: true,
          isLoading: false,
          accessToken: tokens.accessToken,
          refreshToken: tokens.refreshToken,
          tokenExpiresAt: tokens.expiresAt,
        }),
      
      logout: async () => {
        const { accessToken } = get();
        
        // Call backend logout to invalidate refresh token
        if (accessToken) {
          try {
            await fetch(`${API_CONFIG.BASE_URL}/api/v1/auth/logout`, {
              method: 'POST',
              headers: {
                'Authorization': `Bearer ${accessToken}`,
                'Content-Type': 'application/json',
              },
            });
          } catch {
            // Ignore network errors on logout
          }
        }
        
        set({
          user: null,
          isAuthenticated: false,
          isLoading: false,
          accessToken: null,
          refreshToken: null,
          tokenExpiresAt: null,
        });
      },
      
      refreshAccessToken: async () => {
        const { refreshToken } = get();
        if (!refreshToken) return false;
        
        try {
          const response = await fetch(
            `${API_CONFIG.BASE_URL}/api/v1/auth/refresh`,
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ refresh_token: refreshToken }),
            }
          );
          
          if (!response.ok) return false;
          
          const data = await response.json();
          const tokens = data.data;
          
          set({
            accessToken: tokens.access_token,
            refreshToken: tokens.refresh_token,
            tokenExpiresAt: tokens.expires_at,
          });
          
          return true;
        } catch {
          return false;
        }
      },
      
      isTokenExpired: () => {
        const { tokenExpiresAt } = get();
        if (!tokenExpiresAt) return true;
        return new Date(tokenExpiresAt).getTime() - Date.now() < 5 * 60 * 1000;
      },
      
      setTokens: (tokens) =>
        set({
          accessToken: tokens.accessToken,
          refreshToken: tokens.refreshToken,
          tokenExpiresAt: tokens.expiresAt,
        }),
    }),
    {
      name: 'auth-storage',
      // Use AsyncStorage for non-sensitive state, SecureStore for tokens
      // In practice, you might split this into two stores
      storage: createJSONStorage(() => AsyncStorage),
      partialize: (state) => ({
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
);
```

---

## 5. HTTP Client with Auto-Refresh

`src/services/httpClient.ts`:

```typescript
import { API_CONFIG } from '@/config/api';
import { useAuthStore } from '@/store/authStore';

class HttpClient {
  private baseURL: string;
  private isRefreshing = false;
  private refreshSubscribers: Array<(token: string | null) => void> = [];
  
  constructor() {
    this.baseURL = API_CONFIG.BASE_URL;
  }
  
  private getAuthHeaders(): Record<string, string> {
    const { accessToken } = useAuthStore.getState();
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (accessToken) {
      headers['Authorization'] = `Bearer ${accessToken}`;
    }
    return headers;
  }
  
  private async refreshToken(): Promise<string | null> {
    if (this.isRefreshing) {
      return new Promise((resolve) => {
        this.refreshSubscribers.push(resolve);
      });
    }
    
    this.isRefreshing = true;
    
    try {
      const success = await useAuthStore.getState().refreshAccessToken();
      const { accessToken } = useAuthStore.getState();
      
      this.refreshSubscribers.forEach((cb) => cb(accessToken));
      this.refreshSubscribers = [];
      
      return success ? accessToken : null;
    } finally {
      this.isRefreshing = false;
    }
  }
  
  async request<T>(
    endpoint: string,
    options: RequestInit = {},
    retry = true
  ): Promise<T> {
    const url = `${this.baseURL}${endpoint}`;
    const { isTokenExpired } = useAuthStore.getState();
    
    // Proactive refresh if token is expiring soon
    if (isTokenExpired() && retry) {
      const newToken = await this.refreshToken();
      if (!newToken) {
        useAuthStore.getState().logout();
        throw new Error('Session expired');
      }
    }
    
    const response = await fetch(url, {
      ...options,
      headers: {
        ...this.getAuthHeaders(),
        ...(options.headers as Record<string, string> || {}),
      },
    });
    
    // Handle 401 - try refresh once
    if (response.status === 401 && retry) {
      const newToken = await this.refreshToken();
      
      if (newToken) {
        const retryResponse = await fetch(url, {
          ...options,
          headers: {
            ...this.getAuthHeaders(),
            ...(options.headers as Record<string, string> || {}),
          },
        });
        
        if (!retryResponse.ok) {
          throw new Error(`Request failed: ${retryResponse.status}`);
        }
        return retryResponse.json();
      } else {
        useAuthStore.getState().logout();
        throw new Error('Session expired');
      }
    }
    
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.message || `Request failed: ${response.status}`);
    }
    
    return response.json();
  }
  
  get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' });
  }
  
  post<T>(endpoint: string, body: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }
  
  put<T>(endpoint: string, body: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  }
  
  delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }
}

export const httpClient = new HttpClient();
```

---

## 6. API Service Layer

`src/services/api.ts`:

```typescript
import { httpClient } from './httpClient';

// Types
export interface RegisterRequest {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface ApiResponse<T> {
  data: T;
  message?: string;
}

// Auth
export const authApi = {
  register: (data: RegisterRequest) =>
    httpClient.post<ApiResponse<{ id: string; email: string }>>(
      '/api/v1/auth/register',
      data
    ),
  
  login: (data: LoginRequest) =>
    httpClient.post<ApiResponse<{
      access_token: string;
      refresh_token: string;
      expires_at: string;
      user: User;
    }>>('/api/v1/auth/login', data),
  
  verifyEmail: (code: string) =>
    httpClient.post<ApiResponse<void>>('/api/v1/auth/verify-email', { code }),
  
  resendVerification: () =>
    httpClient.post<ApiResponse<void>>('/api/v1/auth/resend-verification', {}),
  
  forgotPassword: (email: string) =>
    httpClient.post<ApiResponse<void>>('/api/v1/auth/forgot-password', { email }),
  
  getProfile: () =>
    httpClient.get<ApiResponse<User>>('/api/v1/users/me'),
  
  updateProfile: (data: Partial<User>) =>
    httpClient.put<ApiResponse<User>>('/api/v1/users/me', data),
};

// Content
export const contentApi = {
  getCourses: () =>
    httpClient.get<ApiResponse<Course[]>>('/api/v1/courses'),
  
  createCourse: (data: Partial<Course>) =>
    httpClient.post<ApiResponse<Course>>('/api/v1/courses', data),
  
  getMaterials: () =>
    httpClient.get<ApiResponse<Material[]>>('/api/v1/materials'),
  
  getQuizzes: () =>
    httpClient.get<ApiResponse<Quiz[]>>('/api/v1/quizzes'),
  
  getFlashcardDecks: () =>
    httpClient.get<ApiResponse<FlashcardDeck[]>>('/api/v1/flashcard-decks'),
};

// AI
export const aiApi = {
  chat: (query: string, contextChunks: string[] = []) =>
    httpClient.post<ApiResponse<{ response: string }>>('/api/v1/ai/chat', {
      query,
      context_chunks: contextChunks,
    }),
  
  generateFlashcards: (topic: string, numCards: number = 10) =>
    httpClient.post<ApiResponse<{ flashcards: Flashcard[] }>>(
      '/api/v1/study/flashcards',
      { topic, num_cards: numCards }
    ),
  
  generateQuiz: (topic: string, quizType: string = 'multiple_choice') =>
    httpClient.post<ApiResponse<Quiz>>('/api/v1/study/quiz', {
      topic,
      quiz_type: quizType,
    }),
  
  transcribeAudio: (audioBase64: string) =>
    httpClient.post<ApiResponse<{ text: string }>>(
      '/api/v1/ai/speech-to-text',
      { audio: audioBase64 }
    ),
  
  generateNotes: (transcription: string) =>
    httpClient.post<ApiResponse<{ notes: string }>>(
      '/api/v1/writing/notes',
      { transcription }
    ),
};

// Analytics
export const analyticsApi = {
  getStudyStreak: () =>
    httpClient.get<ApiResponse<{ streak: number }>>('/api/v1/analytics/study-streak'),
  
  getAIUsage: () =>
    httpClient.get<ApiResponse<{ quota_used: number; quota_limit: number }>>(
      '/api/v1/analytics/ai-usage'
    ),
  
  recordStudySession: (durationMinutes: number) =>
    httpClient.post<ApiResponse<void>>('/api/v1/analytics/study-sessions', {
      duration_minutes: durationMinutes,
    }),
};
```

---

## 7. File Uploads (PDFs, Audio)

The backend uses **presigned URLs** for file uploads. Mobile follows the same 2-step flow as web.

`src/services/upload.ts`:

```typescript
import RNFS from 'react-native-fs';

export const uploadService = {
  /**
   * Upload a file using presigned URL
   */
  async uploadFile(
    filePath: string,
    fileName: string,
    mimeType: string
  ): Promise<string> {
    // Step 1: Request presigned URL from Content Service
    const presignRes = await httpClient.post<ApiResponse<{
      presigned_url: string;
      material_id: string;
      public_url: string;
    }>>('/api/v1/materials', {
      title: fileName,
      content_type: mimeType,
      file_size: await RNFS.stat(filePath).then(s => s.size),
    });
    
    const { presigned_url, public_url } = presignRes.data;
    
    // Step 2: Upload directly to MinIO/S3 using presigned URL
    const fileData = await RNFS.readFile(filePath, 'base64');
    const binaryData = Buffer.from(fileData, 'base64');
    
    const uploadRes = await fetch(presigned_url, {
      method: 'PUT',
      headers: {
        'Content-Type': mimeType,
      },
      body: binaryData,
    });
    
    if (!uploadRes.ok) {
      throw new Error('Upload failed');
    }
    
    return public_url;
  },
  
  /**
   * Pick and upload a PDF document
   */
  async uploadPdf() {
    const DocumentPicker = require('react-native-document-picker');
    
    const result = await DocumentPicker.pick({
      type: [DocumentPicker.types.pdf],
    });
    
    return this.uploadFile(result[0].uri, result[0].name, 'application/pdf');
  },
};
```

---

## 8. Audio Recording & Transcription

`src/services/audio.ts`:

```typescript
import AudioRecorderPlayer from 'react-native-audio-recorder-player';

const audioRecorder = new AudioRecorderPlayer();

export const audioService = {
  async startRecording(): Promise<void> {
    await audioRecorder.startRecorder();
  },
  
  async stopRecording(): Promise<string> {
    const result = await audioRecorder.stopRecorder();
    return result; // Returns file path
  },
  
  async uploadAndTranscribe(filePath: string): Promise<string> {
    // Read file as base64
    const base64Audio = await RNFS.readFile(filePath, 'base64');
    
    // Send to Audio Service via Gateway
    const response = await aiApi.transcribeAudio(base64Audio);
    return response.data.text;
  },
  
  async playAudio(url: string): Promise<void> {
    await audioRecorder.startPlayer(url);
  },
};
```

---

## 9. WebSocket (Real-time Sync)

`src/services/websocket.ts`:

```typescript
import { Client } from '@stomp/stompjs';
import { API_CONFIG } from '@/config/api';
import { useAuthStore } from '@/store/authStore';

class WebSocketClient {
  private client: Client | null = null;
  
  connect() {
    const { accessToken } = useAuthStore.getState();
    if (!accessToken) return;
    
    this.client = new Client({
      brokerURL: `ws://${API_CONFIG.BASE_URL.replace('http://', '').replace('https://', '')}/api/v1/ws`,
      connectHeaders: {
        Authorization: `Bearer ${accessToken}`,
      },
      onConnect: () => {
        console.log('WebSocket connected');
        
        // Subscribe to sync events
        this.client?.subscribe('/user/sync/events', (message) => {
          const event = JSON.parse(message.body);
          this.handleSyncEvent(event);
        });
        
        // Subscribe to presence updates
        this.client?.subscribe('/topic/presence', (message) => {
          const presence = JSON.parse(message.body);
          // Update presence state
        });
      },
      onDisconnect: () => {
        console.log('WebSocket disconnected');
      },
      onStompError: (frame) => {
        console.error('STOMP error:', frame);
      },
    });
    
    this.client.activate();
  }
  
  disconnect() {
    this.client?.deactivate();
    this.client = null;
  }
  
  private handleSyncEvent(event: unknown) {
    // Dispatch to your state management
    console.log('Sync event:', event);
  }
  
  sendPresence(status: 'online' | 'away' | 'offline') {
    this.client?.publish({
      destination: '/app/presence',
      body: JSON.stringify({ status }),
    });
  }
}

export const wsClient = new WebSocketClient();
```

**Note:** If the Sync Service uses raw WebSocket instead of STOMP, use `react-native-websocket` or native `WebSocket`.

---

## 10. Push Notifications (FCM + APNs)

### Backend Setup (already exists)

The Notification Service already supports FCM. You just need to:

1. **Register device token** after login
2. **Handle push payloads** in the app

`src/services/pushNotifications.ts`:

```typescript
import messaging from '@react-native-firebase/messaging';
import { httpClient } from './httpClient';

export const pushNotificationService = {
  async requestPermission(): Promise<boolean> {
    const authStatus = await messaging().requestPermission();
    return authStatus === messaging.AuthorizationStatus.AUTHORIZED ||
           authStatus === messaging.AuthorizationStatus.PROVISIONAL;
  },
  
  async getToken(): Promise<string | null> {
    return messaging().getToken();
  },
  
  async registerDevice(token: string): Promise<void> {
    await httpClient.post('/api/v1/notifications/devices/register', {
      token,
      platform: Platform.OS, // 'ios' or 'android'
    });
  },
  
  async unregisterDevice(token: string): Promise<void> {
    await httpClient.delete(`/api/v1/notifications/devices/${token}`);
  },
  
  setupListeners() {
    // Foreground messages
    messaging().onMessage(async (remoteMessage) => {
      console.log('Foreground notification:', remoteMessage);
      // Show local notification
    });
    
    // Background/quit state tap handler
    messaging().onNotificationOpenedApp((remoteMessage) => {
      console.log('Notification tapped:', remoteMessage);
      // Navigate to relevant screen
    });
    
    // Token refresh
    messaging().onTokenRefresh(async (newToken) => {
      await this.registerDevice(newToken);
    });
  },
};
```

### Firebase Setup

1. Create a Firebase project
2. Add Android + iOS apps
3. Download `google-services.json` (Android) and `GoogleService-Info.plist` (iOS)
4. Place in respective native project folders
5. Set `FIREBASE_SERVICE_ACCOUNT_PATH` in Notification Service env

---

## 11. Screen Structure (React Navigation)

```typescript
// src/navigation/AppNavigator.tsx
import { NavigationContainer } from '@react-navigation/native';
import { createNativeStackNavigator } from '@react-navigation/native-stack';

const Stack = createNativeStackNavigator();

export function AppNavigator() {
  const { isAuthenticated, isLoading } = useAuthStore();
  
  if (isLoading) {
    return <SplashScreen />;
  }
  
  return (
    <NavigationContainer>
      <Stack.Navigator>
        {!isAuthenticated ? (
          // Auth stack
          <>
            <Stack.Screen name="Login" component={LoginScreen} />
            <Stack.Screen name="Register" component={RegisterScreen} />
            <Stack.Screen name="VerifyEmail" component={VerifyEmailScreen} />
            <Stack.Screen name="ForgotPassword" component={ForgotPasswordScreen} />
          </>
        ) : (
          // Main stack
          <>
            <Stack.Screen name="Home" component={HomeScreen} />
            <Stack.Screen name="Courses" component={CoursesScreen} />
            <Stack.Screen name="Materials" component={MaterialsScreen} />
            <Stack.Screen name="Quiz" component={QuizScreen} />
            <Stack.Screen name="Flashcards" component={FlashcardsScreen} />
            <Stack.Screen name="AIChat" component={AIChatScreen} />
            <Stack.Screen name="StudySession" component={StudySessionScreen} />
            <Stack.Screen name="Profile" component={ProfileScreen} />
            <Stack.Screen name="Settings" component={SettingsScreen} />
          </>
        )}
      </Stack.Navigator>
    </NavigationContainer>
  );
}
```

---

## 12. React Query Integration

```typescript
// src/providers/QueryProvider.tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes
      retry: 2,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 30000),
    },
  },
});

// Example hooks
export function useCourses() {
  return useQuery({
    queryKey: ['courses'],
    queryFn: () => contentApi.getCourses(),
  });
}

export function useAIChat() {
  return useMutation({
    mutationFn: (query: string) => aiApi.chat(query),
  });
}

export function useUploadMaterial() {
  return useMutation({
    mutationFn: (filePath: string) => uploadService.uploadPdf(filePath),
  });
}
```

---

## 13. Biometric Authentication (Optional)

```typescript
import ReactNativeBiometrics from 'react-native-biometrics';

export const biometricAuth = {
  async isAvailable(): Promise<boolean> {
    const { available } = await ReactNativeBiometrics.isSensorAvailable();
    return available;
  },
  
  async prompt(): Promise<boolean> {
    const { success } = await ReactNativeBiometrics.simplePrompt({
      promptMessage: 'Authenticate to access Zuri',
    });
    return success;
  },
};
```

---

## 14. Offline Support Strategy

| Feature | Approach |
|---------|----------|
| **Auth token** | SecureStore persists across app restarts |
| **Course list** | React Query caches for 5 min, refetches on reconnect |
| **Quiz attempts** | Queue offline submissions, sync when online |
| **AI chat** | Show cached responses, queue new messages |
| **File uploads** | Queue uploads, retry with exponential backoff |

`src/services/offlineQueue.ts`:

```typescript
import AsyncStorage from '@react-native-async-storage/async-storage';

interface QueuedRequest {
  id: string;
  endpoint: string;
  method: string;
  body: unknown;
  timestamp: number;
}

export const offlineQueue = {
  async enqueue(request: Omit<QueuedRequest, 'id' | 'timestamp'>): Promise<void> {
    const queue = await this.getQueue();
    queue.push({
      ...request,
      id: Math.random().toString(36),
      timestamp: Date.now(),
    });
    await AsyncStorage.setItem('offline_queue', JSON.stringify(queue));
  },
  
  async processQueue(): Promise<void> {
    const queue = await this.getQueue();
    const failed: QueuedRequest[] = [];
    
    for (const request of queue) {
      try {
        await httpClient.request(request.endpoint, {
          method: request.method,
          body: JSON.stringify(request.body),
        });
      } catch {
        failed.push(request);
      }
    }
    
    await AsyncStorage.setItem('offline_queue', JSON.stringify(failed));
  },
  
  async getQueue(): Promise<QueuedRequest[]> {
    const data = await AsyncStorage.getItem('offline_queue');
    return data ? JSON.parse(data) : [];
  },
};
```

---

## 15. Platform-Specific Notes

### Android

```xml
<!-- android/app/src/main/AndroidManifest.xml -->
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.RECORD_AUDIO" />
<uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE" />
<uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE" />
```

- Use `10.0.2.2` for emulator localhost access
- For physical devices, use LAN IP or ngrok

### iOS

```xml
<!-- ios/ZuriMobile/Info.plist -->
<key>NSMicrophoneUsageDescription</key>
<string>We need microphone access for speech-to-text features</string>
<key>NSAppTransportSecurity</key>
<dict>
  <key>NSAllowsArbitraryLoads</key>
  <true/> <!-- Only for development! -->
</dict>
```

- iOS simulator can use `localhost`
- Physical device needs LAN IP or ngrok
- Remove `NSAllowsArbitraryLoads` before App Store submission

---

## 16. Complete Mobile Project Structure

```
ZuriMobile/
├── src/
│   ├── config/
│   │   └── api.ts                    # BASE_URL, timeouts
│   ├── store/
│   │   ├── authStore.ts              # Zustand auth state
│   │   └── secureStorage.ts          # SecureStore wrapper
│   ├── services/
│   │   ├── httpClient.ts             # Fetch + auto-refresh
│   │   ├── api.ts                    # Typed API functions
│   │   ├── upload.ts                 # File upload logic
│   │   ├── audio.ts                  # Recording + playback
│   │   ├── websocket.ts              # Real-time sync
│   │   └── pushNotifications.ts      # FCM/APNs
│   ├── hooks/
│   │   ├── useAuth.ts                # Auth helpers
│   │   ├── useCourses.ts             # React Query hooks
│   │   ├── useAI.ts                  # AI mutation hooks
│   │   └── useStudySession.ts        # Study tracking
│   ├── screens/
│   │   ├── auth/
│   │   │   ├── LoginScreen.tsx
│   │   │   ├── RegisterScreen.tsx
│   │   │   └── VerifyEmailScreen.tsx
│   │   ├── main/
│   │   │   ├── HomeScreen.tsx
│   │   │   ├── CoursesScreen.tsx
│   │   │   ├── MaterialsScreen.tsx
│   │   │   ├── QuizScreen.tsx
│   │   │   ├── FlashcardsScreen.tsx
│   │   │   ├── AIChatScreen.tsx
│   │   │   ├── StudySessionScreen.tsx
│   │   │   └── ProfileScreen.tsx
│   │   └── common/
│   │       └── SplashScreen.tsx
│   ├── navigation/
│   │   └── AppNavigator.tsx
│   ├── types/
│   │   └── index.ts                  # Shared TypeScript types
│   └── utils/
│       └── offlineQueue.ts           # Offline request queue
├── App.tsx                           # Entry point
└── package.json
```

---

## 17. Testing the Integration

### Local Development Checklist

1. **Backend running:** `cd infra && docker-compose up -d`
2. **Gateway healthy:** `curl http://localhost:8080/health`
3. **Mobile app connected:** Check Metro bundler logs
4. **Registration test:** Create account, verify email code appears in logs (or use `BYPASS_EMAIL_VERIFICATION=true`)
5. **Login test:** Get JWT token, store in SecureStore
6. **Protected route test:** Fetch `/api/v1/users/me`
7. **AI endpoint test:** Send chat message, check `X-Quota-Remaining` header
8. **File upload test:** Pick PDF, upload via presigned URL
9. **WebSocket test:** Connect to `/api/v1/ws`, verify real-time updates

### Common Issues

| Issue | Cause | Fix |
|-------|-------|-----|
| `Network request failed` | Wrong BASE_URL | Use `10.0.2.2` (Android emulator), LAN IP (physical device), or ngrok |
| `401 Unauthorized` | Token expired | Ensure auto-refresh logic is working |
| `CORS error` | Missing Origin header | Add `'Origin': 'http://localhost:3000'` to fetch headers |
| `File upload fails` | Wrong content-type | Use exact MIME type from file picker |
| `WebSocket disconnects` | Auth token missing | Pass `Authorization` header in connection |

---

## Summary: What You Need to Build

| Component | Estimated Effort | Reuses Backend? |
|-----------|-----------------|-----------------|
| Auth screens (login, register, verify) | 2 days | Yes - existing endpoints |
| Home dashboard | 1 day | Yes |
| Courses & materials list | 2 days | Yes |
| PDF upload | 1 day | Yes - same presigned URL flow |
| AI Chat screen | 2 days | Yes - same `/api/v1/ai/chat` |
| Flashcards & Quiz screens | 3 days | Yes |
| Study session tracker | 2 days | Yes |
| Audio recording + transcription | 2 days | Yes - same `/api/v1/ai/speech-to-text` |
| Profile & settings | 1 day | Yes |
| Push notifications | 1 day | Yes - backend already supports FCM |
| WebSocket sync | 1 day | Yes - same `/api/v1/ws` |
| **Total** | **~18 days** | **100% backend reuse** |

The backend is already a complete API. Your mobile app is purely a **new client consuming the same Gateway**.
