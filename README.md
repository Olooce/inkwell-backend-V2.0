# Inkwell Backend

![image](https://github.com/user-attachments/assets/24b39c64-7bcf-4b6c-a8f6-9b8b76ff0988)


## Overview
Inkwell Backend is the server-side component of the Inkwell project. It handles user authentication, data storage, story and comic generation, and integrates AI-powered functionality through LLM services. Built in Go, the backend uses a modular architecture that cleanly separates configuration, database management, and business logic.

## Features
- **User Authentication & Authorization:** Secure login, registration, and token management.
- **Story & Comic Generation:** Create, update, and complete stories; generate comics based on stories.
- **AI Integration:** LLM and image generation using external services.
- **Event-Driven Architecture:** Internal event bus to trigger background processes.
- **RESTful API:** RESTful endpoints for client-side integrations.
- **Cross-Platform Support:** OS-specific commands to manage external services (e.g., starting/stopping Ollama).

## Project Structure
```
.
├── cmd
│   └── app
│       ├── main.go                 # Application entry point
│       └── seed.go                 # Database seeding script
├── config-example.xml              # Example configuration file
├── config.xml                      # Application configuration file
├── go.mod                          # Go module file
├── go.sum                          # Dependency checksum file
├── internal
│   ├── config
│   │   └── config.go             # Configuration loader
│   ├── db
│   │   └── connection_manager.go # Database connection management
│   ├── llm
│   │   ├── ollama_client.go      # LLM API client (Ollama)
│   │   └── stableDiffusion_wrapper.go  # Stable Diffusion integration
│   ├── model
│   │   └── model.go             # Data models
│   ├── repository    # Data access layer
│   │   ├── assessment_repository.go  
|   |   ├── question_repository.go  
|   |   ├── story_repository.go  
|   |   └── user_repository.go   
│   └── service
│       ├── analysis_service.go   # Analysis & writing skills
│       ├── assessment_service.go # Assessment logic
│       ├── auth_service.go       # User authentication
│       ├── comic_service.go      # Comic generation
│       ├── progress_service.go   # Progress tracking
│       ├── story_service.go      # Story management
│       └── user_service.go       # User management
├── utilities
│   ├── auth_middleware.go        # JWT authentication middleware
│   ├── CORS_middleware.go        # CORS handling
│   ├── event_bus.go              # Internal event bus
│   └── jwt_util.go               # JWT utility functions
└── working
    ├── comics                  # Generated comic PDFs
    └── storyImages             # AI-generated story images
```

## Getting Started

### Prerequisites
- **Go:** Install the latest version from [Go Downloads](https://go.dev/dl/).
- **Database:** Set up your preferred database (PostgreSQL, MySQL, SQLite, etc.) and configure it in `config.xml`.
- **External Services:** Ensure any external services (e.g., Ollama, Stable Diffusion) are installed and accessible.

### Installation
1. **Clone the Repository:**
   ```bash
   git clone https://github.com/Olooce/inkwell-backend-V2.0.git
   cd inkwell-backend
   ```
2. **Download Dependencies:**
   ```bash
   go mod tidy
   ```
3. **Configure the Application:**
    - Edit `config.xml` with your database credentials, API tokens, and other settings.

### Running the Application
- **Development Mode:**
  ```bash
  go run cmd/app/main.go
  ```
- **Seeding the Database (if needed):**
  ```bash
  go run cmd/app/seed.go
  ```
- **Build and Run Executable:**
  ```bash
  go build -o inkwell cmd/app/main.go
  ./inkwell
  ```

## API Documentation

### Authentication Routes
- **POST `/auth/register`**  
  **Description:** Register a new user.  
  **Request Body Example:**
  ```json
  {
    "name": "John Doe",
    "email": "john@example.com",
    "password": "secret"
  }
  ```
  **Response Example:**
  ```json
  {
    "message": "User registered successfully"
  }
  ```

- **POST `/auth/login`**  
  **Description:** Log in a user and receive a JWT token.  
  **Request Body Example:**
  ```json
  {
    "email": "john@example.com",
    "authhash": "hashed_password"
  }
  ```
  **Response Example:**
  ```json
  {
    "user_id": 1,
    "token": "jwt-token-here"
  }
  ```

- **POST `/auth/refresh`**  
  **Description:** Refresh JWT tokens.  
  **Request Body Example:**
  ```json
  {
    "refresh_token": "refresh-token-here"
  }
  ```
  **Response Example:**
  ```json
  {
    "access_token": "new-jwt-token",
    "refresh_token": "new-refresh-token"
  }
  ```

### User Routes
- **GET `/user`**  
  **Description:** Retrieve all users.  
  **Response Example:**
  ```json
  [
    {
      "id": 1,
      "name": "John Doe",
      "email": "john@example.com",
      "": ""
    }
  ]
  ```

### Assessment Routes
- **POST `/assessments/start`**  
  **Description:** Start a new assessment session with a randomly selected grammar topic.  
  **Response Example:**
  ```json
  {
    "session_id": "abc123",
    "topic": "Tenses",
    "questions": [ /* question objects */ ]
  }
  ```

- **POST `/assessments/submit`**  
  **Description:** Submit an answer for a question within an assessment.  
  **Request Body Example:**
  ```json
  {
    "session_id": "abc123",
    "question_id": 1,
    "answer": "Your answer here"
  }
  ```
  **Response Example:**
  ```json
  {
    "feedback": "Correct",
    "is_correct": true
  }
  ```

- **GET `/assessments/:session_id`**  
  **Description:** Retrieve a specific assessment session by session ID.  
  **Response Example:**
  ```json
  {
    "session_id": "abc123",
    "questions": [ /* question objects */ ],
    "answers": [ /* answer objects */ ]
  }
  ```

### Story Routes
- **GET `/stories/`**  
  **Description:** Retrieve all stories.  
  **Response Example:**
  ```json
  [
    {
      "id": 1,
      "title": "A Great Story",
      "analysis": "Detailed analysis..."
    }
  ]
  ```

- **POST `/stories/start_story`**  
  **Description:** Start a new story.  
  **Request Body Example:**
  ```json
  {
    "title": "A New Adventure"
  }
  ```
  **Response Example:**
  ```json
  {
    "story_id": 1,
    "guidance": "Begin with an exciting sentence!",
    "max_sentences": 5
  }
  ```

- **POST `/stories/:id/add_sentence`**  
  **Description:** Add a sentence to an existing story.  
  **Request Body Example:**
  ```json
  {
    "sentence": "Once upon a time..."
  }
  ```
  **Response Example:**
  ```json
  {
    "sentence": {
      "original_text": "Once upon a time...",
      "corrected_text": "Once upon a time...",
      "feedback": "Looks good!",
      "image_url": "http://example.com/image.png"
    }
  }
  ```

- **POST `/stories/:id/complete_story`**  
  **Description:** Mark a story as complete.  
  **Response Example:**
  ```json
  {
    "message": "Story completed successfully"
  }
  ```

- **GET `/stories/progress`**  
  **Description:** Get the progress of the current user's in-progress story.  
  **Response Example:**
  ```json
  {
    "story_id": 1,
    "progress": "3/5 sentences added"
  }
  ```

- **GET `/stories/comics`**  
  **Description:** Retrieve all generated comics for the authenticated user.  
  **Response Example:**
  ```json
  [
    {
      "comic_id": 10,
      "url": "http://example.com/comic_10.pdf"
    }
  ]
  ```

### Analysis Routes (Writing Skills)
- **GET `/writing-skills/analysis/`**  
  **Description:** Get detailed analysis for completed stories along with writing tips.  
  **Response Example:**
  ```json
  {
    "stories": [
      {
        "story_id": 1,
        "title": "A Great Story",
        "analysis": "Detailed analysis...",
        "tips": ["Tip 1", "Tip 2"]
      }
    ]
  }
  ```

- **GET `/writing-skills/analysis/overview`**  
  **Description:** Retrieve an overview of writing skills progress.  
  **Response Example:**
  ```json
  {
    "initial_progress": "Initial progress data",
    "current_progress": "Current progress data"
  }
  ```

- **GET `/writing-skills/analysis/download_report?type=initial` or `?type=current`**  
  **Description:** Download a PDF report for initial or current progress.  
  **Response:** A PDF file is served with the appropriate download headers.

### Static File & Download Routes
- **GET `/static`**  
  **Description:** Serve static files from the `working` directory.

- **GET `/download/comics/:filename`**  
  **Description:** Download a comic PDF with proper headers.  
  **Example:** Accessing `/download/comics/comic_10.pdf` initiates a download of that comic.

## License
This project is licensed under the **MIT License**.

---
<a href="https://next.ossinsight.io/widgets/official/compose-contributors?repo_id=949105136&limit=200" target="_blank" style="display: block" align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://next.ossinsight.io/widgets/official/compose-contributors/thumbnail.png?repo_id=949105136&limit=200&image_size=auto&color_scheme=dark" width="655" height="auto">
    <img alt="Contributors of Olooce/inkwell-backend-V2.0" src="https://next.ossinsight.io/widgets/official/compose-contributors/thumbnail.png?repo_id=949105136&limit=200&image_size=auto&color_scheme=light" width="655" height="auto">
  </picture>
</a>

