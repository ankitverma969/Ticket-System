# Ticket System API

A robust, production-ready Ticket System backend implemented in **Go** using the **chi router**, **MongoDB** (official Go driver), **JWT authentication**, and interactive **Swagger/OpenAPI documentation**.

## 🌐 Public Deployment URLs
- **Interactive Swagger Documentation**: `http://localhost:8080/docs` (or deployed: `https://<deployed-domain>/docs`)
- **Public Health URL**: `http://localhost:8080/health` (or deployed: `https://<deployed-domain>/health`)

---

## 📖 Interactive Swagger / OpenAPI Documentation

Interactive Swagger UI is available out-of-the-box:

```text
http://localhost:8080/docs
```

The Swagger UI provides:
- Inspection of all 7 API endpoints with request/response schemas.
- Example payloads for tickets, authentication, and status transitions.
- Interactive testing with the **"Try it out"** button.
- Built-in **Authorize** modal supporting `Bearer <JWT>` tokens to test protected ticket endpoints.

### Authentication & Testing Workflow in Swagger UI

1. Start the application (`go run ./cmd/server` or `docker run -p 8080:8080 ticket-system`).
2. Open `http://localhost:8080/docs` in your browser.
3. Scroll to **Authentication** and expand `POST /auth/register` to register a new user account.
4. Expand `POST /auth/login`, click **Try it out**, enter your credentials, and click **Execute**.
5. Copy the returned `token` from the response JSON body.
6. Scroll to the top of the Swagger page and click the green **Authorize** button.
7. Enter `Bearer <your_token>` (or `<your_token>`) and click **Authorize**, then **Close**.
8. Test the protected **Tickets** endpoints:
   - `POST /tickets`: Create a new ticket (ownership is securely derived from JWT).
   - `GET /tickets`: List tickets belonging strictly to the authenticated user.
   - `GET /tickets/{id}`: Inspect a specific ticket.
   - `PATCH /tickets/{id}/status`: Transition ticket status (`open -> in_progress -> closed`).

---

## 📋 API Endpoints

| Method | Endpoint | Auth Required | Description |
| :--- | :--- | :---: | :--- |
| `GET` | `/health` | No | Public health status check |
| `GET` | `/docs` | No | Interactive Swagger UI documentation |
| `POST` | `/auth/register` | No | Register a new user with bcrypt password hashing |
| `POST` | `/auth/login` | No | Login and receive signed HS256 JWT |
| `POST` | `/tickets` | **Bearer JWT** | Create a new ticket (initial status: `open`) |
| `GET` | `/tickets` | **Bearer JWT** | List all tickets owned by authenticated user |
| `GET` | `/tickets/{id}` | **Bearer JWT** | Retrieve a single owned ticket by ID |
| `PATCH`| `/tickets/{id}/status` | **Bearer JWT** | Update status (`open -> in_progress -> closed`) |

---

## 🔒 Security & Ticket Ownership Model

- **Bcrypt Hashing**: User passwords are saved as bcrypt hashes (`bcrypt.DefaultCost`) and never logged or returned in responses.
- **Strict User Ownership**: `user_id` is always derived from the validated JWT token in request context. Client attempts to specify `user_id` or ownership fields are completely ignored.
- **Cross-User Protection**: A user cannot read (`GET /tickets/{id}`) or modify (`PATCH /tickets/{id}/status`) tickets belonging to other users. Attempting to do so returns `404 Not Found` (or `403 Forbidden`) to prevent resource enumeration.
- **Status State Machine**:
  - Valid transitions: `open -> in_progress`, `in_progress -> closed`.
  - Illegal jumps (`open -> closed`, backward transitions) are rejected with `400 Bad Request`.
  - `closed` tickets are strictly terminal and cannot be reopened (`409 Conflict`).

---

## ⚙️ Environment Variables

Copy `.env.example` to `.env` or configure them in your cloud hosting provider dashboard:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | HTTP server port (automatically provided by cloud platforms like Render) |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection URI (use MongoDB Atlas for production) |
| `MONGODB_DATABASE` | `ticket_system` | Target database name |
| `JWT_SECRET` | *(required in production)* | Strong secret key for signing JWT tokens |
| `JWT_EXPIRATION` | `24h` | JWT validity duration (e.g., `24h`, `12h`) |

---

## 🐳 Docker Usage

### 1. Build the Docker Image
```bash
docker build -t ticket-system .
```

### 2. Run the Container
```bash
docker run -p 8080:8080 \
  -e MONGODB_URI="mongodb+srv://<user>:<password>@cluster.mongodb.net/?retryWrites=true&w=majority" \
  -e MONGODB_DATABASE="ticket_system" \
  -e JWT_SECRET="your_secure_random_jwt_secret" \
  -e JWT_EXPIRATION="24h" \
  ticket-system
```

### 3. Verify Health Check and Swagger UI
- Health: `curl http://localhost:8080/health` -> `{"status":"ok"}`
- Swagger: Visit `http://localhost:8080/docs` in your browser.

---

## 💻 Local Development

### Prerequisites
- Go 1.22+
- Local MongoDB running on port 27017 or a MongoDB Atlas connection URI

### Run Locally
```bash
go run ./cmd/server
```

### Run Tests
```bash
go test -v -count=1 ./...
```

---

## 🚀 Deployment Instructions (e.g., Render)

1. **Push code to GitHub**:
   ```bash
   git push origin main
   ```
2. **MongoDB Atlas (Database)**:
   - Create a free cluster on [MongoDB Atlas](https://www.mongodb.com/cloud/atlas).
   - Create a database user and password.
   - Whitelist network access (`0.0.0.0/0` for cloud hosting).
   - Copy the connection string (`mongodb+srv://...`).
3. **Deploy on Render**:
   - Go to [dashboard.render.com](https://dashboard.render.com) and click **New + -> Web Service**.
   - Connect your GitHub repository (`Ticket-System`).
   - Select **Docker** environment (Render automatically detects the multi-stage `Dockerfile`).
   - Add the following **Environment Variables** in Render Settings:
     - `MONGODB_URI`: `mongodb+srv://<username>:<password>@cluster0.mongodb.net/?retryWrites=true&w=majority`
     - `MONGODB_DATABASE`: `ticket_system`
     - `JWT_SECRET`: `<generated_random_secret_string>`
     - `JWT_EXPIRATION`: `24h`
   - Click **Create Web Service**.
4. **Update URLs**:
   - Once deployed, copy your service URL and update the placeholders in this README.
   - Verify `https://<your-service-name>.onrender.com/health` returns `{"status":"ok"}`.
   - Verify `https://<your-service-name>.onrender.com/docs` displays the Swagger UI.

---

## 📖 API Usage Examples via curl

### 1. Register User
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"securepassword123"}'
```
**Response (201 Created)**:
```json
{
  "id": "6aa953fcf3574c54469cddac",
  "email": "alice@example.com"
}
```

### 2. Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"securepassword123"}'
```
**Response (200 OK)**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsIn...",
  "user": {
    "id": "6aa953fcf3574c54469cddac",
    "email": "alice@example.com"
  }
}
```

### 3. Create Ticket
```bash
curl -X POST http://localhost:8080/tickets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"title":"Cannot access dashboard","description":"Getting error 500 on dashboard page"}'
```
**Response (201 Created)**:
```json
{
  "id": "6aa95d646543754963be2c07",
  "user_id": "6aa953fcf3574c54469cddac",
  "title": "Cannot access dashboard",
  "description": "Getting error 500 on dashboard page",
  "status": "open",
  "created_at": "2026-09-15T14:49:48Z",
  "updated_at": "2026-09-15T14:49:48Z"
}
```

### 4. Update Ticket Status
```bash
curl -X PATCH http://localhost:8080/tickets/6aa95d646543754963be2c07/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"status":"in_progress"}'
```
**Response (200 OK)**:
```json
{
  "id": "6aa95d646543754963be2c07",
  "user_id": "6aa953fcf3574c54469cddac",
  "title": "Cannot access dashboard",
  "description": "Getting error 500 on dashboard page",
  "status": "in_progress",
  "created_at": "2026-09-15T14:49:48Z",
  "updated_at": "2026-09-15T14:52:10Z"
}
```
*(Transitions must strictly follow `open -> in_progress -> closed`. Reopening closed tickets returns HTTP 409 Conflict).*
