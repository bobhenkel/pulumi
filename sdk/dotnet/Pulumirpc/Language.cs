// Generated by the protocol buffer compiler.  DO NOT EDIT!
// source: language.proto
#pragma warning disable 1591, 0612, 3021
#region Designer generated code

using pb = global::Google.Protobuf;
using pbc = global::Google.Protobuf.Collections;
using pbr = global::Google.Protobuf.Reflection;
using scg = global::System.Collections.Generic;
namespace Pulumirpc {

  /// <summary>Holder for reflection information generated from language.proto</summary>
  public static partial class LanguageReflection {

    #region Descriptor
    /// <summary>File descriptor for language.proto</summary>
    public static pbr::FileDescriptor Descriptor {
      get { return descriptor; }
    }
    private static pbr::FileDescriptor descriptor;

    static LanguageReflection() {
      byte[] descriptorData = global::System.Convert.FromBase64String(
          string.Concat(
            "Cg5sYW5ndWFnZS5wcm90bxIJcHVsdW1pcnBjGgxwbHVnaW4ucHJvdG8aG2dv",
            "b2dsZS9wcm90b2J1Zi9lbXB0eS5wcm90byJKChlHZXRSZXF1aXJlZFBsdWdp",
            "bnNSZXF1ZXN0Eg8KB3Byb2plY3QYASABKAkSCwoDcHdkGAIgASgJEg8KB3By",
            "b2dyYW0YAyABKAkiSgoaR2V0UmVxdWlyZWRQbHVnaW5zUmVzcG9uc2USLAoH",
            "cGx1Z2lucxgBIAMoCzIbLnB1bHVtaXJwYy5QbHVnaW5EZXBlbmRlbmN5IvUB",
            "CgpSdW5SZXF1ZXN0Eg8KB3Byb2plY3QYASABKAkSDQoFc3RhY2sYAiABKAkS",
            "CwoDcHdkGAMgASgJEg8KB3Byb2dyYW0YBCABKAkSDAoEYXJncxgFIAMoCRIx",
            "CgZjb25maWcYBiADKAsyIS5wdWx1bWlycGMuUnVuUmVxdWVzdC5Db25maWdF",
            "bnRyeRIOCgZkcnlSdW4YByABKAgSEAoIcGFyYWxsZWwYCCABKAUSFwoPbW9u",
            "aXRvcl9hZGRyZXNzGAkgASgJGi0KC0NvbmZpZ0VudHJ5EgsKA2tleRgBIAEo",
            "CRINCgV2YWx1ZRgCIAEoCToCOAEiHAoLUnVuUmVzcG9uc2USDQoFZXJyb3IY",
            "ASABKAky8AEKD0xhbmd1YWdlUnVudGltZRJjChJHZXRSZXF1aXJlZFBsdWdp",
            "bnMSJC5wdWx1bWlycGMuR2V0UmVxdWlyZWRQbHVnaW5zUmVxdWVzdBolLnB1",
            "bHVtaXJwYy5HZXRSZXF1aXJlZFBsdWdpbnNSZXNwb25zZSIAEjYKA1J1bhIV",
            "LnB1bHVtaXJwYy5SdW5SZXF1ZXN0GhYucHVsdW1pcnBjLlJ1blJlc3BvbnNl",
            "IgASQAoNR2V0UGx1Z2luSW5mbxIWLmdvb2dsZS5wcm90b2J1Zi5FbXB0eRoV",
            "LnB1bHVtaXJwYy5QbHVnaW5JbmZvIgBiBnByb3RvMw=="));
      descriptor = pbr::FileDescriptor.FromGeneratedCode(descriptorData,
          new pbr::FileDescriptor[] { global::Pulumirpc.PluginReflection.Descriptor, global::Google.Protobuf.WellKnownTypes.EmptyReflection.Descriptor, },
          new pbr::GeneratedClrTypeInfo(null, new pbr::GeneratedClrTypeInfo[] {
            new pbr::GeneratedClrTypeInfo(typeof(global::Pulumirpc.GetRequiredPluginsRequest), global::Pulumirpc.GetRequiredPluginsRequest.Parser, new[]{ "Project", "Pwd", "Program" }, null, null, null),
            new pbr::GeneratedClrTypeInfo(typeof(global::Pulumirpc.GetRequiredPluginsResponse), global::Pulumirpc.GetRequiredPluginsResponse.Parser, new[]{ "Plugins" }, null, null, null),
            new pbr::GeneratedClrTypeInfo(typeof(global::Pulumirpc.RunRequest), global::Pulumirpc.RunRequest.Parser, new[]{ "Project", "Stack", "Pwd", "Program", "Args", "Config", "DryRun", "Parallel", "MonitorAddress" }, null, null, new pbr::GeneratedClrTypeInfo[] { null, }),
            new pbr::GeneratedClrTypeInfo(typeof(global::Pulumirpc.RunResponse), global::Pulumirpc.RunResponse.Parser, new[]{ "Error" }, null, null, null)
          }));
    }
    #endregion

  }
  #region Messages
  public sealed partial class GetRequiredPluginsRequest : pb::IMessage<GetRequiredPluginsRequest> {
    private static readonly pb::MessageParser<GetRequiredPluginsRequest> _parser = new pb::MessageParser<GetRequiredPluginsRequest>(() => new GetRequiredPluginsRequest());
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pb::MessageParser<GetRequiredPluginsRequest> Parser { get { return _parser; } }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pbr::MessageDescriptor Descriptor {
      get { return global::Pulumirpc.LanguageReflection.Descriptor.MessageTypes[0]; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    pbr::MessageDescriptor pb::IMessage.Descriptor {
      get { return Descriptor; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsRequest() {
      OnConstruction();
    }

    partial void OnConstruction();

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsRequest(GetRequiredPluginsRequest other) : this() {
      project_ = other.project_;
      pwd_ = other.pwd_;
      program_ = other.program_;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsRequest Clone() {
      return new GetRequiredPluginsRequest(this);
    }

    /// <summary>Field number for the "project" field.</summary>
    public const int ProjectFieldNumber = 1;
    private string project_ = "";
    /// <summary>
    /// the project name.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Project {
      get { return project_; }
      set {
        project_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "pwd" field.</summary>
    public const int PwdFieldNumber = 2;
    private string pwd_ = "";
    /// <summary>
    /// the program's working directory.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Pwd {
      get { return pwd_; }
      set {
        pwd_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "program" field.</summary>
    public const int ProgramFieldNumber = 3;
    private string program_ = "";
    /// <summary>
    /// the path to the program.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Program {
      get { return program_; }
      set {
        program_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override bool Equals(object other) {
      return Equals(other as GetRequiredPluginsRequest);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public bool Equals(GetRequiredPluginsRequest other) {
      if (ReferenceEquals(other, null)) {
        return false;
      }
      if (ReferenceEquals(other, this)) {
        return true;
      }
      if (Project != other.Project) return false;
      if (Pwd != other.Pwd) return false;
      if (Program != other.Program) return false;
      return true;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override int GetHashCode() {
      int hash = 1;
      if (Project.Length != 0) hash ^= Project.GetHashCode();
      if (Pwd.Length != 0) hash ^= Pwd.GetHashCode();
      if (Program.Length != 0) hash ^= Program.GetHashCode();
      return hash;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override string ToString() {
      return pb::JsonFormatter.ToDiagnosticString(this);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void WriteTo(pb::CodedOutputStream output) {
      if (Project.Length != 0) {
        output.WriteRawTag(10);
        output.WriteString(Project);
      }
      if (Pwd.Length != 0) {
        output.WriteRawTag(18);
        output.WriteString(Pwd);
      }
      if (Program.Length != 0) {
        output.WriteRawTag(26);
        output.WriteString(Program);
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public int CalculateSize() {
      int size = 0;
      if (Project.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Project);
      }
      if (Pwd.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Pwd);
      }
      if (Program.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Program);
      }
      return size;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(GetRequiredPluginsRequest other) {
      if (other == null) {
        return;
      }
      if (other.Project.Length != 0) {
        Project = other.Project;
      }
      if (other.Pwd.Length != 0) {
        Pwd = other.Pwd;
      }
      if (other.Program.Length != 0) {
        Program = other.Program;
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(pb::CodedInputStream input) {
      uint tag;
      while ((tag = input.ReadTag()) != 0) {
        switch(tag) {
          default:
            input.SkipLastField();
            break;
          case 10: {
            Project = input.ReadString();
            break;
          }
          case 18: {
            Pwd = input.ReadString();
            break;
          }
          case 26: {
            Program = input.ReadString();
            break;
          }
        }
      }
    }

  }

  public sealed partial class GetRequiredPluginsResponse : pb::IMessage<GetRequiredPluginsResponse> {
    private static readonly pb::MessageParser<GetRequiredPluginsResponse> _parser = new pb::MessageParser<GetRequiredPluginsResponse>(() => new GetRequiredPluginsResponse());
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pb::MessageParser<GetRequiredPluginsResponse> Parser { get { return _parser; } }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pbr::MessageDescriptor Descriptor {
      get { return global::Pulumirpc.LanguageReflection.Descriptor.MessageTypes[1]; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    pbr::MessageDescriptor pb::IMessage.Descriptor {
      get { return Descriptor; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsResponse() {
      OnConstruction();
    }

    partial void OnConstruction();

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsResponse(GetRequiredPluginsResponse other) : this() {
      plugins_ = other.plugins_.Clone();
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public GetRequiredPluginsResponse Clone() {
      return new GetRequiredPluginsResponse(this);
    }

    /// <summary>Field number for the "plugins" field.</summary>
    public const int PluginsFieldNumber = 1;
    private static readonly pb::FieldCodec<global::Pulumirpc.PluginDependency> _repeated_plugins_codec
        = pb::FieldCodec.ForMessage(10, global::Pulumirpc.PluginDependency.Parser);
    private readonly pbc::RepeatedField<global::Pulumirpc.PluginDependency> plugins_ = new pbc::RepeatedField<global::Pulumirpc.PluginDependency>();
    /// <summary>
    /// a list of plugins required by this program.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public pbc::RepeatedField<global::Pulumirpc.PluginDependency> Plugins {
      get { return plugins_; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override bool Equals(object other) {
      return Equals(other as GetRequiredPluginsResponse);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public bool Equals(GetRequiredPluginsResponse other) {
      if (ReferenceEquals(other, null)) {
        return false;
      }
      if (ReferenceEquals(other, this)) {
        return true;
      }
      if(!plugins_.Equals(other.plugins_)) return false;
      return true;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override int GetHashCode() {
      int hash = 1;
      hash ^= plugins_.GetHashCode();
      return hash;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override string ToString() {
      return pb::JsonFormatter.ToDiagnosticString(this);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void WriteTo(pb::CodedOutputStream output) {
      plugins_.WriteTo(output, _repeated_plugins_codec);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public int CalculateSize() {
      int size = 0;
      size += plugins_.CalculateSize(_repeated_plugins_codec);
      return size;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(GetRequiredPluginsResponse other) {
      if (other == null) {
        return;
      }
      plugins_.Add(other.plugins_);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(pb::CodedInputStream input) {
      uint tag;
      while ((tag = input.ReadTag()) != 0) {
        switch(tag) {
          default:
            input.SkipLastField();
            break;
          case 10: {
            plugins_.AddEntriesFrom(input, _repeated_plugins_codec);
            break;
          }
        }
      }
    }

  }

  /// <summary>
  /// RunRequest asks the interpreter to execute a program.
  /// </summary>
  public sealed partial class RunRequest : pb::IMessage<RunRequest> {
    private static readonly pb::MessageParser<RunRequest> _parser = new pb::MessageParser<RunRequest>(() => new RunRequest());
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pb::MessageParser<RunRequest> Parser { get { return _parser; } }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pbr::MessageDescriptor Descriptor {
      get { return global::Pulumirpc.LanguageReflection.Descriptor.MessageTypes[2]; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    pbr::MessageDescriptor pb::IMessage.Descriptor {
      get { return Descriptor; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunRequest() {
      OnConstruction();
    }

    partial void OnConstruction();

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunRequest(RunRequest other) : this() {
      project_ = other.project_;
      stack_ = other.stack_;
      pwd_ = other.pwd_;
      program_ = other.program_;
      args_ = other.args_.Clone();
      config_ = other.config_.Clone();
      dryRun_ = other.dryRun_;
      parallel_ = other.parallel_;
      monitorAddress_ = other.monitorAddress_;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunRequest Clone() {
      return new RunRequest(this);
    }

    /// <summary>Field number for the "project" field.</summary>
    public const int ProjectFieldNumber = 1;
    private string project_ = "";
    /// <summary>
    /// the project name.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Project {
      get { return project_; }
      set {
        project_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "stack" field.</summary>
    public const int StackFieldNumber = 2;
    private string stack_ = "";
    /// <summary>
    /// the name of the stack being deployed into.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Stack {
      get { return stack_; }
      set {
        stack_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "pwd" field.</summary>
    public const int PwdFieldNumber = 3;
    private string pwd_ = "";
    /// <summary>
    /// the program's working directory.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Pwd {
      get { return pwd_; }
      set {
        pwd_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "program" field.</summary>
    public const int ProgramFieldNumber = 4;
    private string program_ = "";
    /// <summary>
    /// the path to the program to execute.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Program {
      get { return program_; }
      set {
        program_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    /// <summary>Field number for the "args" field.</summary>
    public const int ArgsFieldNumber = 5;
    private static readonly pb::FieldCodec<string> _repeated_args_codec
        = pb::FieldCodec.ForString(42);
    private readonly pbc::RepeatedField<string> args_ = new pbc::RepeatedField<string>();
    /// <summary>
    /// any arguments to pass to the program.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public pbc::RepeatedField<string> Args {
      get { return args_; }
    }

    /// <summary>Field number for the "config" field.</summary>
    public const int ConfigFieldNumber = 6;
    private static readonly pbc::MapField<string, string>.Codec _map_config_codec
        = new pbc::MapField<string, string>.Codec(pb::FieldCodec.ForString(10), pb::FieldCodec.ForString(18), 50);
    private readonly pbc::MapField<string, string> config_ = new pbc::MapField<string, string>();
    /// <summary>
    /// the configuration variables to apply before running.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public pbc::MapField<string, string> Config {
      get { return config_; }
    }

    /// <summary>Field number for the "dryRun" field.</summary>
    public const int DryRunFieldNumber = 7;
    private bool dryRun_;
    /// <summary>
    /// true if we're only doing a dryrun (preview).
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public bool DryRun {
      get { return dryRun_; }
      set {
        dryRun_ = value;
      }
    }

    /// <summary>Field number for the "parallel" field.</summary>
    public const int ParallelFieldNumber = 8;
    private int parallel_;
    /// <summary>
    /// the degree of parallelism for resource operations (&lt;=1 for serial).
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public int Parallel {
      get { return parallel_; }
      set {
        parallel_ = value;
      }
    }

    /// <summary>Field number for the "monitor_address" field.</summary>
    public const int MonitorAddressFieldNumber = 9;
    private string monitorAddress_ = "";
    /// <summary>
    /// the address for communicating back to the resource monitor.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string MonitorAddress {
      get { return monitorAddress_; }
      set {
        monitorAddress_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override bool Equals(object other) {
      return Equals(other as RunRequest);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public bool Equals(RunRequest other) {
      if (ReferenceEquals(other, null)) {
        return false;
      }
      if (ReferenceEquals(other, this)) {
        return true;
      }
      if (Project != other.Project) return false;
      if (Stack != other.Stack) return false;
      if (Pwd != other.Pwd) return false;
      if (Program != other.Program) return false;
      if(!args_.Equals(other.args_)) return false;
      if (!Config.Equals(other.Config)) return false;
      if (DryRun != other.DryRun) return false;
      if (Parallel != other.Parallel) return false;
      if (MonitorAddress != other.MonitorAddress) return false;
      return true;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override int GetHashCode() {
      int hash = 1;
      if (Project.Length != 0) hash ^= Project.GetHashCode();
      if (Stack.Length != 0) hash ^= Stack.GetHashCode();
      if (Pwd.Length != 0) hash ^= Pwd.GetHashCode();
      if (Program.Length != 0) hash ^= Program.GetHashCode();
      hash ^= args_.GetHashCode();
      hash ^= Config.GetHashCode();
      if (DryRun != false) hash ^= DryRun.GetHashCode();
      if (Parallel != 0) hash ^= Parallel.GetHashCode();
      if (MonitorAddress.Length != 0) hash ^= MonitorAddress.GetHashCode();
      return hash;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override string ToString() {
      return pb::JsonFormatter.ToDiagnosticString(this);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void WriteTo(pb::CodedOutputStream output) {
      if (Project.Length != 0) {
        output.WriteRawTag(10);
        output.WriteString(Project);
      }
      if (Stack.Length != 0) {
        output.WriteRawTag(18);
        output.WriteString(Stack);
      }
      if (Pwd.Length != 0) {
        output.WriteRawTag(26);
        output.WriteString(Pwd);
      }
      if (Program.Length != 0) {
        output.WriteRawTag(34);
        output.WriteString(Program);
      }
      args_.WriteTo(output, _repeated_args_codec);
      config_.WriteTo(output, _map_config_codec);
      if (DryRun != false) {
        output.WriteRawTag(56);
        output.WriteBool(DryRun);
      }
      if (Parallel != 0) {
        output.WriteRawTag(64);
        output.WriteInt32(Parallel);
      }
      if (MonitorAddress.Length != 0) {
        output.WriteRawTag(74);
        output.WriteString(MonitorAddress);
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public int CalculateSize() {
      int size = 0;
      if (Project.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Project);
      }
      if (Stack.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Stack);
      }
      if (Pwd.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Pwd);
      }
      if (Program.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Program);
      }
      size += args_.CalculateSize(_repeated_args_codec);
      size += config_.CalculateSize(_map_config_codec);
      if (DryRun != false) {
        size += 1 + 1;
      }
      if (Parallel != 0) {
        size += 1 + pb::CodedOutputStream.ComputeInt32Size(Parallel);
      }
      if (MonitorAddress.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(MonitorAddress);
      }
      return size;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(RunRequest other) {
      if (other == null) {
        return;
      }
      if (other.Project.Length != 0) {
        Project = other.Project;
      }
      if (other.Stack.Length != 0) {
        Stack = other.Stack;
      }
      if (other.Pwd.Length != 0) {
        Pwd = other.Pwd;
      }
      if (other.Program.Length != 0) {
        Program = other.Program;
      }
      args_.Add(other.args_);
      config_.Add(other.config_);
      if (other.DryRun != false) {
        DryRun = other.DryRun;
      }
      if (other.Parallel != 0) {
        Parallel = other.Parallel;
      }
      if (other.MonitorAddress.Length != 0) {
        MonitorAddress = other.MonitorAddress;
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(pb::CodedInputStream input) {
      uint tag;
      while ((tag = input.ReadTag()) != 0) {
        switch(tag) {
          default:
            input.SkipLastField();
            break;
          case 10: {
            Project = input.ReadString();
            break;
          }
          case 18: {
            Stack = input.ReadString();
            break;
          }
          case 26: {
            Pwd = input.ReadString();
            break;
          }
          case 34: {
            Program = input.ReadString();
            break;
          }
          case 42: {
            args_.AddEntriesFrom(input, _repeated_args_codec);
            break;
          }
          case 50: {
            config_.AddEntriesFrom(input, _map_config_codec);
            break;
          }
          case 56: {
            DryRun = input.ReadBool();
            break;
          }
          case 64: {
            Parallel = input.ReadInt32();
            break;
          }
          case 74: {
            MonitorAddress = input.ReadString();
            break;
          }
        }
      }
    }

  }

  /// <summary>
  /// RunResponse is the response back from the interpreter/source back to the monitor.
  /// </summary>
  public sealed partial class RunResponse : pb::IMessage<RunResponse> {
    private static readonly pb::MessageParser<RunResponse> _parser = new pb::MessageParser<RunResponse>(() => new RunResponse());
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pb::MessageParser<RunResponse> Parser { get { return _parser; } }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public static pbr::MessageDescriptor Descriptor {
      get { return global::Pulumirpc.LanguageReflection.Descriptor.MessageTypes[3]; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    pbr::MessageDescriptor pb::IMessage.Descriptor {
      get { return Descriptor; }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunResponse() {
      OnConstruction();
    }

    partial void OnConstruction();

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunResponse(RunResponse other) : this() {
      error_ = other.error_;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public RunResponse Clone() {
      return new RunResponse(this);
    }

    /// <summary>Field number for the "error" field.</summary>
    public const int ErrorFieldNumber = 1;
    private string error_ = "";
    /// <summary>
    /// an unhandled error if any occurred.
    /// </summary>
    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public string Error {
      get { return error_; }
      set {
        error_ = pb::ProtoPreconditions.CheckNotNull(value, "value");
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override bool Equals(object other) {
      return Equals(other as RunResponse);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public bool Equals(RunResponse other) {
      if (ReferenceEquals(other, null)) {
        return false;
      }
      if (ReferenceEquals(other, this)) {
        return true;
      }
      if (Error != other.Error) return false;
      return true;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override int GetHashCode() {
      int hash = 1;
      if (Error.Length != 0) hash ^= Error.GetHashCode();
      return hash;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public override string ToString() {
      return pb::JsonFormatter.ToDiagnosticString(this);
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void WriteTo(pb::CodedOutputStream output) {
      if (Error.Length != 0) {
        output.WriteRawTag(10);
        output.WriteString(Error);
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public int CalculateSize() {
      int size = 0;
      if (Error.Length != 0) {
        size += 1 + pb::CodedOutputStream.ComputeStringSize(Error);
      }
      return size;
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(RunResponse other) {
      if (other == null) {
        return;
      }
      if (other.Error.Length != 0) {
        Error = other.Error;
      }
    }

    [global::System.Diagnostics.DebuggerNonUserCodeAttribute]
    public void MergeFrom(pb::CodedInputStream input) {
      uint tag;
      while ((tag = input.ReadTag()) != 0) {
        switch(tag) {
          default:
            input.SkipLastField();
            break;
          case 10: {
            Error = input.ReadString();
            break;
          }
        }
      }
    }

  }

  #endregion

}

#endregion Designer generated code