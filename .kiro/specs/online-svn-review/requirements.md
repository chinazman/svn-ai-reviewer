# Requirements Document

## Introduction

本文档定义了SVN代码审核工具的在线版本审核模式功能需求。该功能允许用户直接连接到SVN服务器，搜索和审核指定版本的提交记录，而不仅限于本地工作目录的变更。这将使团队能够审核历史提交、其他开发者的代码，以及远程仓库中的任何版本�?

## Glossary

- **System**: SVN代码审核工具（svn-reviewer�?
- **Online_Mode**: 在线审核模式，直接连接SVN服务器进行版本审�?
- **Local_Mode**: 本地审核模式，审核本地工作目录的变更（现有功能）
- **SVN_Server**: SVN版本控制服务�?
- **Revision**: SVN版本号，标识特定的提�?
- **Commit_Record**: SVN提交记录，包含版本号、作者、日期、消息等信息
- **Credential**: SVN服务器认证凭据，包括用户名和密码
- **Search_Criteria**: 搜索条件，包括目录路径、关键词、用户名�?
- **Batch_Size**: 批量搜索的记录数量，默认100�?

## Requirements

### Requirement 1

**User Story:** 作为开发者，我希望能够通过不同的启动命令进入在线审核模式或本地审核模式，以便根据不同场景选择合适的审核方式

#### Acceptance Criteria

1. WHEN the user executes the command without mode parameter, THE System SHALL start in Local_Mode with GUI interface
2. WHEN the user executes the command with "review online" parameter, THE System SHALL start in Online_Mode
3. WHEN the user executes the command with "review" parameter, THE System SHALL start in Local_Mode with CLI interface
4. THE System SHALL display the current mode name in the user interface
5. THE System SHALL prevent mode switching without restarting the application

### Requirement 2

**User Story:** 作为开发者，我希望系统能够安全地存储SVN服务器凭据，以便不需要每次都输入用户名和密码

#### Acceptance Criteria

1. WHEN the user provides SVN_Server URL, username, and password for the first time, THE System SHALL store the Credential locally in encrypted format
2. WHEN the user starts Online_Mode subsequently, THE System SHALL automatically load the stored Credential
3. THE System SHALL provide an option to update or clear stored Credential
4. THE System SHALL validate Credential by attempting connection to SVN_Server before storing
5. IF Credential validation fails, THEN THE System SHALL display error message and prompt for re-entry
6. THE System SHALL store Credential in a configuration file separate from the main config.yaml

### Requirement 3

**User Story:** 作为开发者，我希望能够通过目录路径、关键词和用户名搜索SVN提交记录，以便快速找到需要审核的版本

#### Acceptance Criteria

1. THE System SHALL provide input fields for directory path, keyword, and username in the search interface
2. WHEN the user leaves directory path empty, THE System SHALL search from the repository root directory
3. WHEN the user provides Search_Criteria, THE System SHALL query SVN_Server for matching Commit_Record entries
4. THE System SHALL display the first 100 Commit_Record entries by default
5. THE System SHALL display each Commit_Record with revision number, author, date, and commit message
6. THE System SHALL support searching with any combination of directory path, keyword, and username filters
7. WHEN no Search_Criteria is provided, THE System SHALL return the most recent 100 Commit_Record entries

### Requirement 4

**User Story:** 作为开发者，我希望能够像TortoiseSVN一样浏览上100条或�?00条提交记录，以便查看更多历史版本

#### Acceptance Criteria

1. WHEN search results are displayed, THE System SHALL show "Previous 100" and "Next 100" navigation buttons
2. WHEN the user clicks "Previous 100" button, THE System SHALL load the previous Batch_Size of Commit_Record entries based on current revision range
3. WHEN the user clicks "Next 100" button, THE System SHALL load the next Batch_Size of Commit_Record entries based on current revision range
4. THE System SHALL disable "Previous 100" button when displaying the most recent revisions
5. THE System SHALL disable "Next 100" button when no older revisions are available
6. THE System SHALL maintain current Search_Criteria when navigating between batches
7. THE System SHALL display the current revision range being viewed

### Requirement 5

**User Story:** 作为开发者，我希望能够在搜索结果中选择多个版本的文件进行审核，以便一次性审核多个相关的提交

#### Acceptance Criteria

1. THE System SHALL display each Commit_Record with a checkbox for selection
2. THE System SHALL allow the user to select multiple Commit_Record entries
3. WHEN the user selects a Commit_Record, THE System SHALL retrieve and display the list of changed files for that revision
4. THE System SHALL display file status indicators (Added, Modified, Deleted) for each file in the selected revisions
5. THE System SHALL allow the user to select individual files from multiple revisions for review
6. THE System SHALL provide a "Select All" option for files within each revision
7. WHEN no files are selected, THE System SHALL disable the "Start Review" button

### Requirement 6

**User Story:** 作为开发者，我希望在线审核模式下的审核流程与本地模式保持一致，以便获得相同的审核体验和报告格式

#### Acceptance Criteria

1. WHEN the user initiates review in Online_Mode, THE System SHALL retrieve file diffs from SVN_Server for selected revisions
2. THE System SHALL use the same AI client and review logic as Local_Mode
3. THE System SHALL generate HTML reports with the same format as Local_Mode
4. THE System SHALL display real-time review progress in the log area
5. THE System SHALL handle errors in the same manner as Local_Mode
6. THE System SHALL support the same AI configuration options as Local_Mode
7. THE System SHALL include revision numbers in the review report for Online_Mode

### Requirement 7

**User Story:** 作为开发者，我希望系统能够处理SVN服务器连接错误和超时，以便在网络不稳定时获得清晰的错误提�?

#### Acceptance Criteria

1. WHEN SVN_Server connection fails, THE System SHALL display a user-friendly error message with the failure reason
2. WHEN SVN_Server operation times out after 30 seconds, THE System SHALL cancel the operation and notify the user
3. THE System SHALL provide a "Retry" option when connection errors occur
4. THE System SHALL validate SVN_Server URL format before attempting connection
5. IF authentication fails, THEN THE System SHALL prompt the user to re-enter Credential
6. THE System SHALL log all SVN_Server communication errors for troubleshooting

### Requirement 8

**User Story:** 作为开发者，我希望在GUI界面中能够方便地切换和使用在线审核功能，以便保持一致的用户体验

#### Acceptance Criteria

1. WHEN the user starts the System in Online_Mode, THE System SHALL display a dedicated online review interface
2. THE System SHALL provide a clear visual distinction between Online_Mode and Local_Mode interfaces
3. THE System SHALL display SVN_Server connection status in the interface
4. THE System SHALL show the currently connected repository URL
5. THE System SHALL provide input fields for all Search_Criteria in a user-friendly layout
6. THE System SHALL display search results in a scrollable table with sortable columns
7. THE System SHALL maintain responsive design principles for all screen sizes
