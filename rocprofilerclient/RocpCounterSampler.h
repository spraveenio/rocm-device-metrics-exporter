// MIT License
//
// Copyright (c) 2025 Advanced Micro Devices, Inc. All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

#ifndef ROCP_COUNTER_SAMPLER_H_
#define ROCP_COUNTER_SAMPLER_H_

#include <rocprofiler-sdk/fwd.h>
#include <rocprofiler-sdk/registration.h>
#include <rocprofiler-sdk/rocprofiler.h>

#include <map>
#include <memory>
#include <unordered_map>
#include <vector>

#define ROCP_ERROR 0
#define ROCP_INFO 1
#define ROCP_DEBUG 2

#define ROCP_LOG(debug_level, msg)                                                             \
  do {                                                                                        \
      std::clog << __FILE__ <<  __LINE__ << " " << msg \
                           << std::endl;                                                      \
  } while (0)

typedef struct metric {
  double value;
} metric;

typedef struct gpu_metric {
  struct metric *list;
} gpu_metric;

namespace amd {
namespace rocp {
class CounterSampler {
 public:
  // Setup system profiling for an agent
  explicit CounterSampler(rocprofiler_agent_id_t agent);

  ~CounterSampler();

  // Decode the counter name of a record
  const std::string& decode_record_name(const rocprofiler_record_counter_t& rec) const;

  // Get the dimensions of a record (what CU/SE/etc the counter is for). High cost operation
  // should be cached if possible.
  std::unordered_map<std::string, size_t> get_record_dimensions(
      const rocprofiler_record_counter_t& rec);

  // Sample the counter values for a set of counters, returns the records in the out parameter.
  void sample_counter_values(const std::vector<std::string>& counters,
                             std::vector<rocprofiler_record_counter_t>& out, uint64_t duration);

  rocprofiler_agent_id_t get_agent() const { return agent_; }

  // Get the supported counters for an agent
  static std::unordered_map<std::string, rocprofiler_counter_id_t> get_supported_counters(
      rocprofiler_agent_id_t agent);

  // Get the available agents on the system
  static std::vector<rocprofiler_agent_v0_t> get_available_agents();

  static std::vector<std::shared_ptr<CounterSampler>>& get_samplers();
  static int runSample(std::vector<std::string> &metric_fields);

 private:
  rocprofiler_agent_id_t agent_ = {};
  rocprofiler_context_id_t ctx_ = {};
  rocprofiler_buffer_id_t buf_ = {};
  rocprofiler_profile_config_id_t profile_ = {.handle = 0};

  std::map<std::vector<std::string>, rocprofiler_profile_config_id_t> cached_profiles_;
  std::map<uint64_t, uint64_t> profile_sizes_;

  // Internal function used to set the profile for the agent when start_context is called
  void set_profile(rocprofiler_context_id_t ctx, rocprofiler_agent_set_profile_callback_t cb) const;

  // Get the size of a counter in number of records
  size_t get_counter_size(rocprofiler_counter_id_t counter);

  // Get the dimensions of a counter
  std::vector<rocprofiler_record_dimension_info_t> get_counter_dimensions(
      rocprofiler_counter_id_t counter);

  static std::vector<std::shared_ptr<CounterSampler>> samplers_;
};

}  // namespace rocp
}  // namespace amd

#endif  // ROCP_COUNTER_SAMPLER_H_
