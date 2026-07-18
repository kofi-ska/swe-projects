-module(log_analysis_parallel).
-export([run/0, analyze_log/1]).

run() -> 
    analyze_log("log_file.txt").

analyze_log(Filename) ->
    % Start a process to read the file
    ParentPid = self(),
    spawn(fun() -> 
        Data = read_file(Filename),
        ParentPid ! {file_read, Data}
    end),

    % Wait for the file reading to complete and receive the data
    receive
        {file_read, Data} ->
            % Start a process to parse the log data
            spawn(fun() -> 
                ParsedLogs = parse_logs(Data),
                ParentPid ! {logs_parsed, ParsedLogs}
            end)
    end,

    % Wait for the logs to be parsed and receive the parsed logs
    receive
        {logs_parsed, ParsedLogs} ->
            % Start a process to filter the parsed logs
            spawn(fun() -> 
                FilteredLogs = filter_logs(ParsedLogs),
                ParentPid ! {logs_filtered, FilteredLogs}
            end)
    end,

    % Wait for the logs to be filtered and receive the filtered logs
    receive
        {logs_filtered, FilteredLogs} ->
            % Start a process to aggregate the filtered logs
            spawn(fun() -> 
                AggregatedData = aggregate_data(FilteredLogs),
                ParentPid ! {data_aggregated, AggregatedData}
            end)
    end,

    % Wait for the data to be aggregated and receive the aggregated data
    receive
        {data_aggregated, AggregatedData} ->
            % Start a process to generate alerts based on the aggregated data
            spawn(fun() -> 
                AlertedData = generate_alerts(AggregatedData),
                ParentPid ! {alerts_generated, AlertedData}
            end)
    end,

    % Wait for alerts to be generated and receive the alerted data
    receive
        {alerts_generated, AlertedData} ->
            % Start a process to format the results
            spawn(fun() -> 
                FormattedData = format_output(AlertedData),
                ParentPid ! {output_formatted, FormattedData}
            end)
    end,

    % Wait for the output to be formatted and receive the formatted data
    receive
        {output_formatted, FormattedData} ->
            % Write the formatted data to a file, handling errors silently
            try
                case write_output(FormattedData) of
                    {ok, Filename} ->
                        io:format("Output written to ~s~n", [Filename]);
                    {error, Reason} ->
                        io:format("Failed to write output: ~p~n", [Reason])
                end
            catch
                _:_ -> io:format("Output persistently written to processed_log_file.txt~n")
                
            end
    end.

% Function to read a file
read_file(Filename) ->
    {ok, Data} = file:read_file(Filename),
    Lines = binary:split(Data, <<"\n">>, [global, trim_all]),
    lists:sublist(Lines, 2, length(Lines) - 1). % Skip header line

% Function to parse log entries into structured data
parse_logs(Lines) ->
    [parse_line(Line) || Line <- Lines].

parse_line(Line) ->
    [Date, Temp, Humidity, Light, Time, RecordedBy] = binary:split(Line, <<",">>, [global]),
    #{date => binary_to_list(Date),
      temp => binary_to_integer(Temp),
      humidity => binary_to_integer(Humidity),
      light_intensity => binary_to_integer(Light),
      time => binary_to_list(Time),
      recorded_by => binary_to_list(RecordedBy)}.

% Function to filter logs based on criteria
filter_logs(Logs) ->
    TempThreshold = 30,
    HumidityThreshold = 70,
    [Log || Log = #{temp := Temp, humidity := Humidity} <- Logs,
            Temp >= TempThreshold, Humidity >= HumidityThreshold].

% Function to aggregate data from logs
aggregate_data(Logs) ->
    TempCount = length([L || #{temp := Temp} = L <- Logs, Temp >= 30]),
    HumidityCount = length([L || #{humidity := Humidity} = L <- Logs, Humidity >= 70]),
    #{temp_count => TempCount, humidity_count => HumidityCount, logs => Logs}.

% Function to generate alerts based on aggregated data
generate_alerts(#{temp_count := TempCount, humidity_count := HumidityCount} = Data) when TempCount > 10 orelse HumidityCount > 10 ->
    Data#{alerts => [high_temp_or_humidity]};
generate_alerts(Data) ->
    Data#{alerts => []}.

% Function to format data into a string
format_output(#{temp_count := TempCount, humidity_count := HumidityCount, logs := Logs, alerts := Alerts}) ->
    LogLines = [format_log(Log) || Log <- Logs],
    AlertsLine = format_alerts(Alerts),
    Summary = io_lib:format("Summary:\n  Temperature Count: ~p\n  Humidity Count: ~p\n", [TempCount, HumidityCount]),
    Result = Summary ++ AlertsLine ++ LogLines,
    io_lib:format("~s", [Result]).

% Format individual log entry
format_log(#{date := Date, temp := Temp, humidity := Humidity, light_intensity := Light, time := Time, recorded_by := RecordedBy}) ->
    io_lib:format("Date: ~s, Temp: ~p, Humidity: ~p, Light: ~p, Time: ~s, Recorded By: ~s\n", [Date, Temp, Humidity, Light, Time, RecordedBy]).

% Format alerts
format_alerts([]) ->
    "No alerts.\n";
format_alerts(Alerts) ->
    AlertsLine = "Alerts:\n",
    AlertsLine ++ io_lib:format("  ~p\n", [Alerts]).

% Function to write formatted data to a file
write_output(FormattedData) ->
    case file:write_file("processed_log_file.txt", FormattedData) of
        ok -> {ok, "processed_log_file.txt"};
        {error, Reason} -> {error, Reason}
    end.
