1. **Login via OTP** 

API Contract

- Request OTP
    
    
    | **Method** | POST |  |
    | --- | --- | --- |
    | **Endpoint** | {host}}/auth/v1/otp/request |  |
    | **Header** | Content-Type: application/json |  |
    | **REQ** | `{ "phone": "08123456789"}` |  |
    | **RES** | Success 
    
    `{ "status": true, "code": "SUCCESS", "message": "OTP sent successfully", "errorMessage": null, "errorData": null, "data": { "expiresAt": "2026-10-21T12:01:00Z", "cooldownSeconds": 60 }}`
    
    Error 
    
    `{ "status": false, "code": "RATE_LIMITED", "message": "Too many requests", "errorMessage": "You have reached the maximum OTP request limit for this hour.", "errorData": { "retryAfter": 3600 }, "data": nulll }` |  |
- Validate OTP
    
    
    | **Method** | POST |  |
    | --- | --- | --- |
    | **Endpoint** | {{host}}/auth/v1/otp/verify |  |
    | **Header** | Content-Type: application/json |  |
    | **REQ** | `{ "phone": "08123456789", "otp": "123456"}` |  |
    | **RES** | Success 
    
    `{ "status": true, "code": "SUCCESS", "message": "Login successful", "errorMessage": null, "errorData": null, "data": { "accessToken": "eyJhbGciOi...", "refreshToken": "eyJhbGciOi...", "expiresIn": 3600 }}`
    
    Error 
    
    `{ "status": false, "code": "INVALID_OTP", "message": "Verification failed", "errorMessage": "The OTP you entered is incorrect. 2 attempts remaining.", "errorData": { "attemptsRemaining": 2 }, "data": {}}` |  |
- Get Customer
    
    
    | **Method** | POST |  |
    | --- | --- | --- |
    | **Endpoint** | {{host}}/customer/private/customers?phone={phone} |  |
    | **Header** | Content-Type: application/json
    x-api-key: {{secret}} |  |
    | **REQ** | `{ "phone": "08123456789", "otp": "123456"}` |  |
    | **RES** | Success 
    
    `{ "status": true, "code": "SUCCESS", "message": "Customer retrieved", "errorMessage": null, "errorData": null, "data": { "customerId": "c_01HQX...", "phone": "628123456789", "isVerified": false }}`
    
    Error 
    
    `{ "status": false, "code": "INVALID_OTP", "message": "Verification failed", "errorMessage": "The OTP you entered is incorrect. 2 attempts remaining.", "errorData": { "attemptsRemaining": 2 }, "data": {}}` |  |
- Mark Customer as Verified
    
    
    | **Method** | POST |  |
    | --- | --- | --- |
    | **Endpoint** | {{host}}/customer/v1/internal/{customerId}/verify |  |
    | **Header** | Content-Type: application/json
    x-api-key: {{secret}} |  |
    | **REQ** | `{ "verifiedAt": "2026-10-21T12:00:30Z"}` |  |
    | **RES** | Success 
    
    `{ "status": true, "code": "SUCCESS", "message": "Customer verified successfully", "errorMessage": null, "errorData": null, "data": { "customerId": "c_01HQX...", "phoneVerifiedAt": "2026-10-21T12:00:30Z" }}` |  |