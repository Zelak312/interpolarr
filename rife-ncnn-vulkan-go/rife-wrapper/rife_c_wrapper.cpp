#include "rife_c_wrapper.h"
#include "rife.h"
#include <string>

// Implementation of the RifeWrapped class
class RifeWrapped : public RIFE
{
public:
    RifeWrapped(int gpuid, bool _tta_mode, bool _tta_temporal_mode, bool _uhd_mode,
                int _num_threads, bool _rife_v2, bool _rife_v4, int _padding)
        : RIFE(gpuid, _tta_mode, _tta_temporal_mode, _uhd_mode,
               _num_threads, _rife_v2, _rife_v4, _padding)
    {
    }

    int load(const std::string &modeldir)
    {
        return RIFE::load(modeldir);
    }

    int process_frames(unsigned char *data0, unsigned char *data1,
                       int width, int height, int elempack,
                       unsigned char *out_data, float timestep)
    {
        ncnn::Mat inmat0(width, height, data0, elempack, elempack);
        ncnn::Mat inmat1(width, height, data1, elempack, elempack);
        ncnn::Mat outmat(width, height, out_data, elempack, elempack);
        return RIFE::process(inmat0, inmat1, timestep, outmat);
    }
};

// C interface implementations
extern "C"
{
    Rife_Ctx *rife_create(int gpuid, int tta_mode, int tta_temporal_mode, int uhd_mode,
                          int num_threads, int rife_v2, int rife_v4, int padding)
    {
        RifeWrapped *rife = new (std::nothrow) RifeWrapped(
            gpuid, tta_mode != 0, tta_temporal_mode != 0, uhd_mode != 0,
            num_threads, rife_v2 != 0, rife_v4 != 0, padding);

        return reinterpret_cast<Rife_Ctx *>(rife);
    }

    void rife_destroy(Rife_Ctx *ctx)
    {
        if (ctx)
        {
            delete reinterpret_cast<RifeWrapped *>(ctx);
        }
    }

    int rife_load(Rife_Ctx *ctx, const char *modeldir)
    {
        if (!ctx || !modeldir)
            return -1;

        RifeWrapped *rife = reinterpret_cast<RifeWrapped *>(ctx);
        return rife->load(std::string(modeldir));
    }

    int rife_process_frames(Rife_Ctx *ctx,
                            unsigned char *data0, unsigned char *data1,
                            int width, int height, int elempack,
                            unsigned char *out_data, float timestep)
    {
        if (!ctx || !data0 || !data1 || !out_data)
            return -1;

        RifeWrapped *rife = reinterpret_cast<RifeWrapped *>(ctx);
        return rife->process_frames(data0, data1, width, height, elempack, out_data, timestep);
    }

    int rife_get_gpu_count(void)
    {
        return ncnn::get_gpu_count();
    }
} // extern "C"