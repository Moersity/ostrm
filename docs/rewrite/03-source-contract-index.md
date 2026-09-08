# 原项目接口、字段与测试索引

基线：`e25c766753c758f00aa47818057d3d9df0b4969a`。共 62 个控制器 HTTP 路由；静态提取，不代表已运行验证。实现者必须补充 HTTP 状态码、业务 code、默认值、鉴权和响应样例。

## HTTP 路由

| 方法 | 路径 | 源码定位 |
|---|---|---|
| POST | `/api/data-report/event` | [DataReportController.java:35](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/DataReportController.java#L35) |
| POST | `/api/data-report/events` | [DataReportController.java:53](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/DataReportController.java#L53) |
| GET | `/api/logs/{logType}` | [LogController.java:63](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L63) |
| GET | `/api/logs/{logType}/tail` | [LogController.java:81](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L81) |
| GET | `/api/logs/{logType}/download` | [LogController.java:114](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L114) |
| GET | `/api/logs/{logType}/stats` | [LogController.java:154](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L154) |
| POST | `/api/logs/frontend` | [LogController.java:183](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L183) |
| DELETE | `/api/logs/{logType}` | [LogController.java:212](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/LogController.java#L212) |
| GET | `/api/media-servers` | [MediaServerController.java:31](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L31) |
| POST | `/api/media-servers` | [MediaServerController.java:37](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L37) |
| PUT | `/api/media-servers/{id}` | [MediaServerController.java:44](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L44) |
| DELETE | `/api/media-servers/{id}` | [MediaServerController.java:51](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L51) |
| POST | `/api/media-servers/test` | [MediaServerController.java:57](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L57) |
| POST | `/api/media-servers/{id}/test` | [MediaServerController.java:63](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L63) |
| GET | `/api/media-servers/{id}/libraries` | [MediaServerController.java:69](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L69) |
| POST | `/api/media-servers/{id}/refresh` | [MediaServerController.java:75](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/MediaServerController.java#L75) |
| GET | `/api/openlist-config` | [OpenlistConfigController.java:57](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L57) |
| GET | `/api/openlist-config/active` | [OpenlistConfigController.java:67](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L67) |
| GET | `/api/openlist-config/{id}` | [OpenlistConfigController.java:77](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L77) |
| GET | `/api/openlist-config/username/{username}` | [OpenlistConfigController.java:89](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L89) |
| POST | `/api/openlist-config` | [OpenlistConfigController.java:101](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L101) |
| PUT | `/api/openlist-config/{id}` | [OpenlistConfigController.java:112](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L112) |
| DELETE | `/api/openlist-config/{id}` | [OpenlistConfigController.java:125](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L125) |
| PATCH | `/api/openlist-config/{id}/status` | [OpenlistConfigController.java:134](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L134) |
| POST | `/api/openlist-config/validate` | [OpenlistConfigController.java:145](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L145) |
| POST | `/api/openlist-config/validate-path` | [OpenlistConfigController.java:166](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/OpenlistConfigController.java#L166) |
| POST | `/api/auth/sign-in` | [SignController.java:54](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L54) |
| POST | `/api/auth/sign-up` | [SignController.java:95](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L95) |
| GET | `/api/auth/check-user` | [SignController.java:124](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L124) |
| POST | `/api/auth/sign-out` | [SignController.java:164](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L164) |
| POST | `/api/auth/refresh` | [SignController.java:194](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L194) |
| GET | `/api/auth/validate` | [SignController.java:237](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L237) |
| POST | `/api/auth/change-password` | [SignController.java:284](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SignController.java#L284) |
| GET | `/api/system/config` | [SystemConfigController.java:34](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SystemConfigController.java#L34) |
| POST | `/api/system/config` | [SystemConfigController.java:47](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SystemConfigController.java#L47) |
| POST | `/api/system/test-notification` | [SystemConfigController.java:100](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SystemConfigController.java#L100) |
| POST | `/api/system/test-ai-config` | [SystemConfigController.java:114](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/SystemConfigController.java#L114) |
| GET | `/api/task-config` | [TaskConfigController.java:55](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L55) |
| GET | `/api/task-config/active` | [TaskConfigController.java:65](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L65) |
| GET | `/api/task-config/scheduled` | [TaskConfigController.java:75](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L75) |
| GET | `/api/task-config/{id}` | [TaskConfigController.java:85](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L85) |
| GET | `/api/task-config/task-name/{taskName}` | [TaskConfigController.java:97](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L97) |
| GET | `/api/task-config/path` | [TaskConfigController.java:109](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L109) |
| POST | `/api/task-config` | [TaskConfigController.java:121](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L121) |
| PUT | `/api/task-config/{id}` | [TaskConfigController.java:138](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L138) |
| DELETE | `/api/task-config/{id}` | [TaskConfigController.java:157](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L157) |
| PATCH | `/api/task-config/{id}/status` | [TaskConfigController.java:166](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L166) |
| PATCH | `/api/task-config/{id}/last-exec-time` | [TaskConfigController.java:176](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L176) |
| POST | `/api/task-config/{id}/submit` | [TaskConfigController.java:186](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L186) |
| POST | `/api/task-config/{id}/structure-check` | [TaskConfigController.java:198](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L198) |
| GET | `/api/task-config/{id}/structure-check/directories` | [TaskConfigController.java:206](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L206) |
| POST | `/api/task-config/{id}/structure-check/directory` | [TaskConfigController.java:214](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L214) |
| GET | `/api/task-config/{id}/manual-scraping/tree` | [TaskConfigController.java:225](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L225) |
| GET | `/api/task-config/{id}/manual-scraping/tree/children` | [TaskConfigController.java:233](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L233) |
| POST | `/api/task-config/{id}/manual-scraping/preview` | [TaskConfigController.java:243](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L243) |
| POST | `/api/task-config/{id}/manual-scraping/execute` | [TaskConfigController.java:252](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L252) |
| GET | `/api/task-config/{id}/manual-scraping/jobs/latest` | [TaskConfigController.java:261](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L261) |
| GET | `/api/task-config/{id}/manual-scraping/jobs/{jobId}` | [TaskConfigController.java:269](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L269) |
| POST | `/api/task-config/{id}/manual-scraping/jobs/{jobId}/retry` | [TaskConfigController.java:278](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/TaskConfigController.java#L278) |
| GET | `/api/version/check` | [VersionController.java:29](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/VersionController.java#L29) |
| GET | `/api/version/latest` | [VersionController.java:51](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/VersionController.java#L51) |
| DELETE | `/api/version/cache/clear` | [VersionController.java:72](https://github.com/hienao/ostrm/blob/e25c766753c758f00aa47818057d3d9df0b4969a/backend/src/main/java/com/hienao/openlist2strm/controller/VersionController.java#L72) |

## DTO 与实体字段索引

以下仅提取 private 字段用于导航；嵌套 record、校验注解、继承字段必须读原文件。Java Boolean 的 null 与 false 不应合并。

### `backend/src/main/java/com/hienao/openlist2strm/dto/ApiResponse.java`

```text
int code
String message
T data
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/DataReportRequest.java`

```text
String apiKey
String event
Map<String, Object> properties
String timestamp
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/FrontendLogRequest.java`

```text
List<LogEntry> logs
String level
String message
Long timestamp
String userAgent
String url
String userId
String sessionId
Object extra
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/LogReadResponse.java`

```text
List<String> lines
long cursor
String fileKey
boolean reset
boolean hasMore
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/PageRequestDto.java`

```text
int page
int size
Map<String, Direction> sortBy
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/PageResponseDto.java`

```text
long total
T data
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/media/AiRecognitionResult.java`

```text
boolean success
String type
String reason
String title
List<String> titleCandidates
String year
Integer season
Integer episode
String filename
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/media/MediaInfo.java`

```text
MediaType type
String title
String year
Integer season
Integer episode
String originalFileName
String cleanTitle
boolean hasYear
boolean hasSeasonEpisode
int confidence
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/media/MediaServerDtos.java`

```text
String name
String serverType
String apiBaseUrl
String apiKey
Boolean isActive
Long id
String name
String serverType
String apiBaseUrl
boolean apiKeyConfigured
boolean active
LocalDateTime createdAt
LocalDateTime updatedAt
String serverName
String version
String productName
int libraryCount
String id
String name
String collectionType
List<String> locations
String scope
String libraryId
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/openlist/OpenlistConfigDto.java`

```text
Long id
String baseUrl
String token
String basePath
String username
LocalDateTime createdAt
LocalDateTime updatedAt
Boolean isActive
String strmBaseUrl
Boolean enableUrlEncoding
Integer fsApiQpmLimit
Integer fsApiQpsLimit
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/sign/ChangePasswordDto.java`

```text
String oldPassword
String newPassword
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/sign/SignInDto.java`

```text
String username
String password
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/sign/SignUpDto.java`

```text
String username
String password
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/task/ManualScrapingDtos.java`

```text
Long taskId
String taskName
String libraryType
String rootPath
DirectoryNode tree
String name
String path
int videoFileCount
boolean childrenLoaded
List<DirectoryNode> children
String directoryPath
String title
String year
Integer tmdbId
String directoryPath
String mediaType
boolean matched
String searchTitle
String searchYear
String matchMessage
Integer tmdbId
String title
String originalTitle
String year
String overview
Double voteAverage
String posterUrl
String backdropUrl
int videoFileCount
String proposedDirectoryName
List<RenameItem> proposedDirectoryRenames
List<RenameItem> proposedFileRenames
List<String> generatedFiles
List<String> renamedGeneratedFiles
String sourcePath
String sourceName
String targetName
String directoryPath
String mediaType
Integer tmdbId
boolean renameMedia
String finalDirectoryPath
int renamedDirectoryCount
int renamedFileCount
List<String> uploadedFiles
String message
Long id
Long taskId
String directoryPath
String finalDirectoryPath
String mediaType
Integer tmdbId
boolean renameMedia
String status
String stage
int progress
String message
String errorMessage
int renamedDirectoryCount
int renamedFileCount
List<String> uploadedFiles
LocalDateTime createdAt
LocalDateTime startedAt
LocalDateTime completedAt
LocalDateTime updatedAt
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/task/TaskConfigDto.java`

```text
Long id
String taskName
String path
Long openlistConfigId
Boolean needScrap
String libraryType
Boolean skipInvalidStructure
String renameRegex
Boolean autoRenameMedia
Long mediaServerConfigId
String mediaRefreshScope
String mediaLibraryId
String mediaLibraryName
String cron
Boolean isIncrement
String strmPath
Long lastExecTime
LocalDateTime createdAt
LocalDateTime updatedAt
Boolean isActive
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/task/TaskStructureCheckOverview.java`

```text
Long taskId
String taskName
String libraryType
String rootPath
String expectedStructure
boolean supported
String message
TaskStructureCheckResult rootFilesResult
List<DirectoryItem> directories
String name
String path
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/task/TaskStructureCheckResult.java`

```text
Long taskId
String taskName
String libraryType
String rootPath
String expectedStructure
boolean supported
int scannedEntryCount
int videoFileCount
int invalidFileCount
String message
StructureNode tree
String name
String path
String type
String reason
List<StructureNode> children
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/tmdb/TmdbMovieDetail.java`

```text
Integer id
String title
String originalTitle
String overview
String posterPath
String backdropPath
String releaseDate
Integer runtime
List<Genre> genres
List<ProductionCompany> productionCompanies
List<ProductionCountry> productionCountries
List<SpokenLanguage> spokenLanguages
String originalLanguage
Boolean adult
Long budget
Long revenue
Double popularity
Double voteAverage
Integer voteCount
String status
String tagline
String homepage
String imdbId
Integer id
String name
Integer id
String name
String logoPath
String originCountry
String iso31661
String name
String englishName
String iso6391
String name
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/tmdb/TmdbSearchResponse.java`

```text
Integer page
List<TmdbSearchResult> results
Integer totalResults
Integer totalPages
Integer id
String title
String name
String originalTitle
String originalName
String overview
String posterPath
String backdropPath
String releaseDate
String firstAirDate
String mediaType
Boolean adult
String originalLanguage
Double popularity
Double voteAverage
Integer voteCount
List<Integer> genreIds
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/tmdb/TmdbTvDetail.java`

```text
Integer id
String name
String originalName
String overview
String posterPath
String backdropPath
String firstAirDate
String lastAirDate
List<TmdbMovieDetail.Genre> genres
List<TmdbMovieDetail.ProductionCompany> productionCompanies
List<TmdbMovieDetail.ProductionCountry> productionCountries
List<TmdbMovieDetail.SpokenLanguage> spokenLanguages
String originalLanguage
Boolean adult
Double popularity
Double voteAverage
Integer voteCount
String status
String type
String homepage
Boolean inProduction
Integer numberOfSeasons
Integer numberOfEpisodes
List<Integer> episodeRunTime
List<Season> seasons
List<Creator> createdBy
List<Network> networks
List<String> originCountry
Integer id
String name
String overview
String posterPath
Integer seasonNumber
Integer episodeCount
String airDate
Integer id
String name
String creditId
Integer gender
String profilePath
Integer id
String name
String logoPath
String originCountry
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/version/GitHubAsset.java`

```text
String id
String name
String label
String contentType
long size
long downloadCount
String createdAt
String updatedAt
String browserDownloadUrl
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/version/GitHubRelease.java`

```text
String id
String name
String tagName
String body
boolean draft
boolean prerelease
LocalDateTime createdAt
LocalDateTime publishedAt
String htmlUrl
```

### `backend/src/main/java/com/hienao/openlist2strm/dto/version/VersionCheckResponse.java`

```text
String currentVersion
String latestVersion
boolean hasUpdate
String releaseUrl
String releaseNotes
LocalDateTime checkTime
boolean prerelease
LocalDateTime publishedAt
String error
```

### `backend/src/main/java/com/hienao/openlist2strm/entity/ManualScrapingJob.java`

```text
Long id
Long taskId
String directoryPath
String finalDirectoryPath
String mediaType
Integer tmdbId
Boolean renameMedia
String status
String stage
Integer progress
String message
String errorMessage
String renamePlan
Integer renameOperationIndex
Integer renamedDirectoryCount
Integer renamedFileCount
String uploadedFiles
LocalDateTime createdAt
LocalDateTime startedAt
LocalDateTime completedAt
LocalDateTime updatedAt
```

### `backend/src/main/java/com/hienao/openlist2strm/entity/MediaServerConfig.java`

```text
Long id
String name
String serverType
String apiBaseUrl
String apiKey
Boolean isActive
LocalDateTime createdAt
LocalDateTime updatedAt
```

### `backend/src/main/java/com/hienao/openlist2strm/entity/OpenlistConfig.java`

```text
Long id
String baseUrl
String token
String basePath
String username
LocalDateTime createdAt
LocalDateTime updatedAt
Boolean isActive
String strmBaseUrl
Boolean enableUrlEncoding
Integer fsApiQpmLimit
Integer fsApiQpsLimit
```

### `backend/src/main/java/com/hienao/openlist2strm/entity/TaskConfig.java`

```text
Long id
String taskName
String path
Long openlistConfigId
Boolean needScrap
String libraryType
Boolean skipInvalidStructure
String renameRegex
Boolean autoRenameMedia
Long mediaServerConfigId
String mediaRefreshScope
String mediaLibraryId
String mediaLibraryName
String cron
Boolean isIncrement
String strmPath
Long lastExecTime
LocalDateTime createdAt
LocalDateTime updatedAt
Boolean isActive
```

## Java 基线测试清单

移植断言和输入样例；不能通过删除失败用例来达成兼容。

- `backend/src/test/java/com/hienao/openlist2strm/handler/SingleFileHandlerSkipTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/integration/cache/CacheTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/notification/NotificationRendererTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/AiFileNameRecognitionServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/LogServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/ManualScrapingJobServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/ManualScrapingServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/MediaServerApiServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/NotificationServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/OpenlistApiRateLimiterTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/OpenlistApiServiceDownloadTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/OpenlistApiServiceRateLimitTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/OpenlistConfigServiceRateLimitTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/QuartzSchedulerServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/SystemConfigServiceCacheTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/TaskConfigServiceStructureOptionTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/TaskExecutionServiceStructureFilterTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/TaskManifestServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/service/TaskStructureCheckServiceTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/unit/JwtUnitTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/unit/PageRequestDtoUnitTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/util/SeasonDirectoryNameParserTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/util/TaskDirectoryStructureValidatorTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/util/TaskMediaParserTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/util/TmdbIdExtractorTest.java`
- `backend/src/test/java/com/hienao/openlist2strm/util/UrlEncoderTest.java`

## 数据迁移脚本清单

- `backend/src/main/resources/db/migration/V1_0_0__init_schema.sql`
- `backend/src/main/resources/db/migration/V1_0_10__add_fs_api_qps_limit_column.sql`
- `backend/src/main/resources/db/migration/V1_0_11__create_manual_scraping_job_table.sql`
- `backend/src/main/resources/db/migration/V1_0_12__add_skip_invalid_structure_column.sql`
- `backend/src/main/resources/db/migration/V1_0_13__add_manual_scraping_rename_checkpoint.sql`
- `backend/src/main/resources/db/migration/V1_0_14__add_auto_rename_media_column.sql`
- `backend/src/main/resources/db/migration/V1_0_15__add_media_server_refresh.sql`
- `backend/src/main/resources/db/migration/V1_0_1__insert_urp_table.sql`
- `backend/src/main/resources/db/migration/V1_0_2__init_quartz_table.sql`
- `backend/src/main/resources/db/migration/V1_0_3__create_openlist_config_table.sql`
- `backend/src/main/resources/db/migration/V1_0_4__create_task_config_table.sql`
- `backend/src/main/resources/db/migration/V1_0_5__modify_need_rename_to_rename_regex.sql`
- `backend/src/main/resources/db/migration/V1_0_6__add_strm_base_url_column.sql`
- `backend/src/main/resources/db/migration/V1_0_7__add_enable_url_encoding_column.sql`
- `backend/src/main/resources/db/migration/V1_0_8__add_fs_api_qpm_limit_column.sql`
- `backend/src/main/resources/db/migration/V1_0_9__add_task_library_type.sql`
