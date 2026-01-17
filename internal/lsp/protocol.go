package lsp

// LSP Protocol Types based on LSP 3.17
// Reference: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/

// Request and Response IDs
type ID int

// Base JSON-RPC Types
type RequestMessage struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      ID            `json:"id"`
	Method  string        `json:"method"`
	Params  interface{}   `json:"params,omitempty"`
}

type ResponseMessage struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      ID            `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

type NotificationMessage struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  interface{}   `json:"params,omitempty"`
}

type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	ServerNotInitialized = -32002
	UnknownErrorCode = -32001
	RequestCancelled = -32800
)

// LSP Initialization

type InitializeParams struct {
	ProcessID int                `json:"processId"`
	ClientInfo *ClientInfo       `json:"clientInfo,omitempty"`
	Locale    string             `json:"locale,omitempty"`
	RootPath  string             `json:"rootPath,omitempty"`
	RootURI   string             `json:"rootUri,omitempty"`
	Capabilities ClientCapabilities `json:"capabilities"`
	InitializationOptions interface{} `json:"initializationOptions,omitempty"`
	Trace    string             `json:"trace,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
	Workspace       *WorkspaceClientCapabilities       `json:"workspace,omitempty"`
	TextDocument    *TextDocumentClientCapabilities    `json:"textDocument,omitempty"`
	Window          *WindowClientCapabilities          `json:"window,omitempty"`
	General         *GeneralClientCapabilities         `json:"general,omitempty"`
}

type WorkspaceClientCapabilities struct {
	ApplyEdit          bool                              `json:"applyEdit,omitempty"`
	WorkspaceEdit      *WorkspaceEditCapabilities        `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationCapabilities `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles *DidChangeWatchedFilesCapabilities `json:"didChangeWatchedFiles,omitempty"`
	Symbol             *WorkspaceSymbolCapabilities      `json:"symbol,omitempty"`
	ExecuteCommand     *ExecuteCommandCapabilities       `json:"executeCommand,omitempty"`
	Configuration      bool                              `json:"configuration,omitempty"`
	WorkspaceFolders   bool                              `json:"workspaceFolders,omitempty"`
	SemanticTokens     *SemanticTokensWorkspaceCapabilities `json:"semanticTokens,omitempty"`
	CodeLens           *CodeLensWorkspaceCapabilities    `json:"codeLens,omitempty"`
	FileOperations     *FileOperationWorkspaceCapabilities `json:"fileOperations,omitempty"`
}

type WorkspaceEditCapabilities struct {
	DocumentChanges bool `json:"documentChanges,omitempty"`
	ResourceOperations []string `json:"resourceOperations,omitempty"`
	FailureHandling string `json:"failureHandling,omitempty"`
	NormalizesLineEndings bool `json:"normalizesLineEndings,omitempty"`
	ChangeAnnotationSupport *ChangeAnnotationWorkspaceCapabilities `json:"changeAnnotationSupport,omitempty"`
}

type DidChangeConfigurationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DidChangeWatchedFilesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	RelativePatternSupport bool `json:"relativePatternSupport,omitempty"`
}

type WorkspaceSymbolCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	SymbolKind *SymbolKindCapabilities `json:"symbolKind,omitempty"`
	TagSupport *TagSupport `json:"tagSupport,omitempty"`
	ResolveSupport *ResolveSupport `json:"resolveSupport,omitempty"`
}

type ExecuteCommandCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type SymbolKindCapabilities struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type TagSupport struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type ResolveSupport struct {
	Properties []string `json:"properties,omitempty"`
}

type SemanticTokensWorkspaceCapabilities struct {
	RefreshSupport bool `json:"refreshSupport,omitempty"`
}

type CodeLensWorkspaceCapabilities struct {
	RefreshSupport bool `json:"refreshSupport,omitempty"`
}

type FileOperationWorkspaceCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	DidCreate bool `json:"didCreate,omitempty"`
	DidRename bool `json:"didRename,omitempty"`
	DidDelete bool `json:"didDelete,omitempty"`
	WillCreate bool `json:"willCreate,omitempty"`
	WillRename bool `json:"willRename,omitempty"`
	WillDelete bool `json:"willDelete,omitempty"`
}

type ChangeAnnotationWorkspaceCapabilities struct {
	GroupsOnLabel bool `json:"groupsOnLabel,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization          *SynchronizationCapabilities            `json:"synchronization,omitempty"`
	Completion               *CompletionCapabilities                  `json:"completion,omitempty"`
	Hover                    *HoverCapabilities                      `json:"hover,omitempty"`
	SignatureHelp            *SignatureHelpCapabilities              `json:"signatureHelp,omitempty"`
	Declaration              *DeclarationCapabilities                `json:"declaration,omitempty"`
	Definition               *DefinitionCapabilities                 `json:"definition,omitempty"`
	TypeDefinition           *TypeDefinitionCapabilities             `json:"typeDefinition,omitempty"`
	Implementation           *ImplementationCapabilities             `json:"implementation,omitempty"`
	References               *ReferencesCapabilities                 `json:"references,omitempty"`
	DocumentHighlight        *DocumentHighlightCapabilities          `json:"documentHighlight,omitempty"`
	DocumentSymbol           *DocumentSymbolCapabilities             `json:"documentSymbol,omitempty"`
	CodeAction               *CodeActionCapabilities                 `json:"codeAction,omitempty"`
	CodeLens                 *CodeLensCapabilities                   `json:"codeLens,omitempty"`
	DocumentLink             *DocumentLinkCapabilities               `json:"documentLink,omitempty"`
	ColorProvider            *ColorProviderCapabilities              `json:"colorProvider,omitempty"`
	Formatting               *FormattingCapabilities                 `json:"formatting,omitempty"`
	RangeFormatting          *RangeFormattingCapabilities            `json:"rangeFormatting,omitempty"`
	OnTypeFormatting         *OnTypeFormattingCapabilities           `json:"onTypeFormatting,omitempty"`
	Rename                   *RenameCapabilities                     `json:"rename,omitempty"`
	PublishDiagnostics       *PublishDiagnosticsCapabilities         `json:"publishDiagnostics,omitempty"`
	FoldingRange             *FoldingRangeCapabilities               `json:"foldingRange,omitempty"`
	SelectionRange           *SelectionRangeCapabilities             `json:"selectionRange,omitempty"`
	CallHierarchy            *CallHierarchyCapabilities              `json:"callHierarchy,omitempty"`
	SemanticTokens           *SemanticTokensCapabilities             `json:"semanticTokens,omitempty"`
	LinkedEditingRange       *LinkedEditingRangeCapabilities         `json:"linkedEditingRange,omitempty"`
	Moniker                  *MonikerCapabilities                    `json:"moniker,omitempty"`
	TypeHierarchy            *TypeHierarchyCapabilities              `json:"typeHierarchy,omitempty"`
	InlineValue              *InlineValueCapabilities                `json:"inlineValue,omitempty"`
	InlayHint                *InlayHintCapabilities                  `json:"inlayHint,omitempty"`
	Diagnostic               *DiagnosticCapabilities                 `json:"diagnostic,omitempty"`
	InlineCompletion         *InlineCompletionCapabilities           `json:"inlineCompletion,omitempty"`
}

type SynchronizationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type CompletionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	CompletionItem      *CompletionItemCapabilities `json:"completionItem,omitempty"`
	CompletionItemKind  *CompletionItemKindCapabilities `json:"completionItemKind,omitempty"`
	ContextSupport      bool `json:"contextSupport,omitempty"`
	InsertTextMode      *InsertTextModeCapabilities `json:"insertTextMode,omitempty"`
	CompletionList      *CompletionListCapabilities `json:"completionList,omitempty"`
}

type CompletionItemCapabilities struct {
	SnippetSupport          bool `json:"snippetSupport,omitempty"`
	CommitCharactersSupport bool `json:"commitCharactersSupport,omitempty"`
	DocumentationFormat     []string `json:"documentationFormat,omitempty"`
	DeprecatedSupport       bool `json:"deprecatedSupport,omitempty"`
	PreselectSupport        bool `json:"preselectSupport,omitempty"`
	TagSupport              *TagSupport `json:"tagSupport,omitempty"`
	ResolveSupport          *ResolveSupport `json:"resolveSupport,omitempty"`
	InsertTextModeSupport   *InsertTextModeCapabilities `json:"insertTextModeSupport,omitempty"`
	LabelDetailsSupport     bool `json:"labelDetailsSupport,omitempty"`
}

type InsertTextModeCapabilities struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type CompletionItemKindCapabilities struct {
	ValueSet []int `json:"valueSet,omitempty"`
}

type CompletionListCapabilities struct {
	SnippetSupport bool `json:"snippetSupport,omitempty"`
}

type HoverCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

type SignatureHelpCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	SignatureInformation *SignatureInformationCapabilities `json:"signatureInformation,omitempty"`
	ContextSupport       bool `json:"contextSupport,omitempty"`
}

type SignatureInformationCapabilities struct {
	DocumentationFormat       []string `json:"documentationFormat,omitempty"`
	ParameterInformation      *ParameterInformationCapabilities `json:"parameterInformation,omitempty"`
	ActiveParameterSupport    bool `json:"activeParameterSupport,omitempty"`
}

type ParameterInformationCapabilities struct {
	LabelOffsetSupport bool `json:"labelOffsetSupport,omitempty"`
}

type DeclarationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type DefinitionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type TypeDefinitionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type ImplementationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type ReferencesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentHighlightCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentSymbolCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	SymbolKind          *SymbolKindCapabilities `json:"symbolKind,omitempty"`
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
	TagSupport          *TagSupport `json:"tagSupport,omitempty"`
	LabelSupport        bool `json:"labelSupport,omitempty"`
}

type CodeActionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	CodeActionLiteralSupport *CodeActionLiteralSupportCapabilities `json:"codeActionLiteralSupport,omitempty"`
	IsPreferredSupport  bool `json:"isPreferredSupport,omitempty"`
	DisabledSupport     bool `json:"disabledSupport,omitempty"`
	DataSupport         bool `json:"dataSupport,omitempty"`
	ResolveSupport      *ResolveSupport `json:"resolveSupport,omitempty"`
}

type CodeActionLiteralSupportCapabilities struct {
	CodeActionKind *CodeActionKindCapabilities `json:"codeActionKind,omitempty"`
}

type CodeActionKindCapabilities struct {
	ValueSet []string `json:"valueSet,omitempty"`
}

type CodeLensCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentLinkCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	TooltipSupport      bool `json:"tooltipSupport,omitempty"`
}

type ColorProviderCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type FormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type RangeFormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type OnTypeFormattingCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type RenameCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	PrepareSupport      bool `json:"prepareSupport,omitempty"`
	PrepareSupportDefaultBehavior bool `json:"prepareSupportDefaultBehavior,omitempty"`
	HonorsChangeAnnotations bool `json:"honorsChangeAnnotations,omitempty"`
}

type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
	TagSupport         *TagSupport `json:"tagSupport,omitempty"`
	VersionSupport     bool `json:"versionSupport,omitempty"`
	CodeDescriptionSupport bool `json:"codeDescriptionSupport,omitempty"`
	DataSupport        bool `json:"dataSupport,omitempty"`
}

type FoldingRangeCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	RangeLimit          *int `json:"rangeLimit,omitempty"`
	LineFoldingOnly     bool `json:"lineFoldingOnly,omitempty"`
}

type SelectionRangeCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type CallHierarchyCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type SemanticTokensCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	Requests            *SemanticTokensRequestCapabilities `json:"requests,omitempty"`
	TokenTypes          []string `json:"tokenTypes,omitempty"`
	TokenModifiers      []string `json:"tokenModifiers,omitempty"`
	Formats             []string `json:"formats,omitempty"`
	OverlappingTokenSupport bool `json:"overlappingTokenSupport,omitempty"`
	MultilineTokenSupport    bool `json:"multilineTokenSupport,omitempty"`
}

type SemanticTokensRequestCapabilities struct {
	Range  *bool `json:"range,omitempty"`
	Full   *SemanticTokensFullCapabilities `json:"full,omitempty"`
}

type SemanticTokensFullCapabilities struct {
	Delta bool `json:"delta,omitempty"`
}

type LinkedEditingRangeCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type MonikerCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type TypeHierarchyCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type InlineValueCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type InlayHintCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	ResolveSupport      *ResolveSupport `json:"resolveSupport,omitempty"`
}

type DiagnosticCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	RelatedDocumentSupport bool `json:"relatedDocumentSupport,omitempty"`
}

type InlineCompletionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WindowClientCapabilities struct {
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
	ShowMessage      *ShowMessageRequestClientCapabilities `json:"showMessage,omitempty"`
	ShowDocument     *ShowDocumentClientCapabilities `json:"showDocument,omitempty"`
}

type ShowMessageRequestClientCapabilities struct {
	MessageActionItem *MessageActionItemCapabilities `json:"messageActionItem,omitempty"`
}

type MessageActionItemCapabilities struct {
	AdditionalPropertiesSupport bool `json:"additionalPropertiesSupport,omitempty"`
}

type ShowDocumentClientCapabilities struct {
	Support bool `json:"support,omitempty"`
}

type GeneralClientCapabilities struct {
	StaleRequestSupport *StaleRequestSupportCapabilities `json:"staleRequestSupport,omitempty"`
	RegularExpressions  *RegularExpressionsCapabilities  `json:"regularExpressions,omitempty"`
	Markdown            *MarkdownCapabilities            `json:"markdown,omitempty"`
	PositionEncodings   []string                        `json:"positionEncodings,omitempty"`
}

type StaleRequestSupportCapabilities struct {
	Cancel  bool     `json:"cancel,omitempty"`
	ReloadOn []string `json:"reloadOn,omitempty"`
}

type RegularExpressionsCapabilities struct {
	Engine   string `json:"engine,omitempty"`
	Version  string `json:"version,omitempty"`
}

type MarkdownCapabilities struct {
	Parser string `json:"parser,omitempty"`
	Version string `json:"version,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ServerCapabilities struct {
	PositionEncoding       string                       `json:"positionEncoding,omitempty"`
	TextDocumentSync       interface{}                  `json:"textDocumentSync,omitempty"`
	NotebookDocumentSync   interface{}                  `json:"notebookDocumentSync,omitempty"`
	CompletionProvider     *CompletionOptions           `json:"completionProvider,omitempty"`
	HoverProvider          interface{}                  `json:"hoverProvider,omitempty"`
	SignatureHelpProvider  *SignatureHelpOptions        `json:"signatureHelpProvider,omitempty"`
	DeclarationProvider    interface{}                  `json:"declarationProvider,omitempty"`
	DefinitionProvider     interface{}                  `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider interface{}                  `json:"typeDefinitionProvider,omitempty"`
	ImplementationProvider interface{}                  `json:"implementationProvider,omitempty"`
	ReferencesProvider     interface{}                  `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider interface{}               `json:"documentHighlightProvider,omitempty"`
	DocumentSymbolProvider interface{}                  `json:"documentSymbolProvider,omitempty"`
	CodeActionProvider     interface{}                  `json:"codeActionProvider,omitempty"`
	CodeLensProvider       interface{}                  `json:"codeLensProvider,omitempty"`
	DocumentLinkProvider   *DocumentLinkOptions         `json:"documentLinkProvider,omitempty"`
	ColorProvider          interface{}                  `json:"colorProvider,omitempty"`
	DocumentFormattingProvider interface{}              `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider interface{}          `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
	RenameProvider          interface{}                  `json:"renameProvider,omitempty"`
	FoldingRangeProvider    interface{}                  `json:"foldingRangeProvider,omitempty"`
	ExecuteCommandProvider  *ExecuteCommandOptions       `json:"executeCommandProvider,omitempty"`
	SelectionRangeProvider  interface{}                  `json:"selectionRangeProvider,omitempty"`
	CallHierarchyProvider   interface{}                  `json:"callHierarchyProvider,omitempty"`
	LinkedEditingRangeProvider interface{}               `json:"linkedEditingRangeProvider,omitempty"`
	SemanticTokensProvider  interface{}                  `json:"semanticTokensProvider,omitempty"`
	MonoProvider            interface{}                  `json:"monikerProvider,omitempty"`
	TypeHierarchyProvider   interface{}                  `json:"typeHierarchyProvider,omitempty"`
	InlineValueProvider     interface{}                  `json:"inlineValueProvider,omitempty"`
	InlayHintProvider       interface{}                  `json:"inlayHintProvider,omitempty"`
	DiagnosticProvider      interface{}                  `json:"diagnosticProvider,omitempty"`
	Workspace               *ServerCapabilitiesWorkspace `json:"workspace,omitempty"`
	Experimental            interface{}                  `json:"experimental,omitempty"`
}

type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	AllCommitCharacters []string `json:"allCommitCharacters,omitempty"`
	WorkDoneProgress  bool     `json:"workDoneProgress,omitempty"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
	WorkDoneProgress  bool     `json:"workDoneProgress,omitempty"`
}

type DocumentLinkOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
	WorkDoneProgress bool `json:"workDoneProgress,omitempty"`
}

type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

type ServerCapabilitiesWorkspace struct {
	WorkspaceFolders *ServerCapabilitiesWorkspaceFolders `json:"workspaceFolders,omitempty"`
	FileOperations   *FileOperationServerCapabilities   `json:"fileOperations,omitempty"`
}

type ServerCapabilitiesWorkspaceFolders struct {
	Supported bool `json:"supported,omitempty"`
	ChangeNotifications string `json:"changeNotifications,omitempty"`
}

type FileOperationServerCapabilities struct {
	DidCreate   *FileOperationRegistrationOptions `json:"didCreate,omitempty"`
	WillCreate  *FileOperationRegistrationOptions `json:"willCreate,omitempty"`
	DidRename   *FileOperationRegistrationOptions `json:"didRename,omitempty"`
	WillRename  *FileOperationRegistrationOptions `json:"willRename,omitempty"`
	DidDelete   *FileOperationRegistrationOptions `json:"didDelete,omitempty"`
	WillDelete  *FileOperationRegistrationOptions `json:"willDelete,omitempty"`
}

type FileOperationRegistrationOptions struct {
	Filters []FileOperationFilter `json:"filters"`
}

type FileOperationFilter struct {
	Scheme   string `json:"scheme,omitempty"`
	Pattern  FileOperationPattern `json:"pattern"`
}

type FileOperationPattern struct {
	Glob string `json:"glob"`
	Matches string `json:"matches,omitempty"`
	Options *FileOperationPatternOptions `json:"options,omitempty"`
}

type FileOperationPatternOptions struct {
	IgnoreCase bool `json:"ignoreCase,omitempty"`
}

// Text Document Synchronization

type TextDocumentSyncOptions struct {
	OpenClose         bool                     `json:"openClose,omitempty"`
	Change            *TextDocumentSyncKind    `json:"change,omitempty"`
	WillSave          bool                     `json:"willSave,omitempty"`
	WillSaveWaitUntil bool                     `json:"willSaveWaitUntil,omitempty"`
	DidSave           *DidSaveOptions          `json:"didSave,omitempty"`
}

type DidSaveOptions struct {
	IncludeText bool `json:"includeText,omitempty"`
}

type TextDocumentSyncKind int

const (
	TextDocumentSyncKindNone TextDocumentSyncKind = 0
	TextDocumentSyncKindFull TextDocumentSyncKind = 1
	TextDocumentSyncKindIncremental TextDocumentSyncKind = 2
)

type TextDocumentSyncOptionsOrKind struct {
	Options *TextDocumentSyncOptions
	Kind    TextDocumentSyncKind
}

// Text Document Items

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type OptionalVersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version *int   `json:"version,omitempty"`
}

// DidOpen Notification

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChange Notification

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength int    `json:"rangeLength,omitempty"`
	RangeOffset int    `json:"rangeOffset,omitempty"`
	Text        string `json:"text"`
}

// DidClose Notification

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidSave Notification

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

// Positions and Locations

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Completion

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

type CompletionContext struct {
	TriggerKind      int                `json:"triggerKind"`
	TriggerCharacter string             `json:"triggerCharacter,omitempty"`
}

type CompletionItem struct {
	Label               string             `json:"label"`
	Kind                int                `json:"kind,omitempty"`
	Tags                []int              `json:"tags,omitempty"`
	Detail              string             `json:"detail,omitempty"`
	Documentation       interface{}        `json:"documentation,omitempty"`
	Deprecated          bool               `json:"deprecated,omitempty"`
	Preselect           bool               `json:"preselect,omitempty"`
	SortText            string             `json:"sortText,omitempty"`
	FilterText          string             `json:"filterText,omitempty"`
	InsertText          string             `json:"insertText,omitempty"`
	InsertTextFormat    int                `json:"insertTextFormat,omitempty"`
	InsertTextMode      int                `json:"insertTextMode,omitempty"`
	TextEdit            *TextEdit          `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit         `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string           `json:"commitCharacters,omitempty"`
	Command             *Command           `json:"command,omitempty"`
	Data                interface{}        `json:"data,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type TextEdit struct {
	Range Range `json:"range"`
	NewText string `json:"newText"`
}

type Command struct {
	Title     string                 `json:"title"`
	Command   string                 `json:"command"`
	Arguments []interface{}          `json:"arguments,omitempty"`
}

// Definition

type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	WorkDoneProgressParams
	PartialResultParams
}

type WorkDoneProgressParams struct {
	WorkDoneToken interface{} `json:"workDoneToken,omitempty"`
}

type PartialResultParams struct {
	PartialResultToken interface{} `json:"partialResultToken,omitempty"`
}

// Hover

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	WorkDoneProgressParams
}

type Hover struct {
	Contents interface{} `json:"contents"`
	Range    *Range      `json:"range,omitempty"`
}

// Diagnostics

type PublishDiagnosticsParams struct {
	URI         string        `json:"uri"`
	Version     int           `json:"version"`
	Diagnostics []Diagnostic  `json:"diagnostics"`
}

type Diagnostic struct {
	Range              Range           `json:"range"`
	Severity           int             `json:"severity,omitempty"`
	Code               interface{}     `json:"code,omitempty"`
	CodeDescription    *CodeDescription `json:"codeDescription,omitempty"`
	Source             string          `json:"source,omitempty"`
	Message            string          `json:"message"`
	Tags               []int           `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               interface{}     `json:"data,omitempty"`
}

type CodeDescription struct {
	Href string `json:"href"`
}

type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string    `json:"message"`
}

// Cancel Request

type CancelParams struct {
	ID ID `json:"id"`
}

// Shutdown
type ShutdownParams struct {}
type ShutdownResult struct {}
